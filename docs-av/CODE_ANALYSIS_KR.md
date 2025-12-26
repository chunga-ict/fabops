# Fabops 코드 분석 및 개선 보고서

## 1. 개요

본 문서는 Fabops 아키텍처 전환 작업(Phase 1-6)에서 구현된 코드를 정밀 분석하고, 완전체(Production-Ready)가 되기 위한 개선점을 도출한다.

### 분석 대상
| 모듈 | 파일 | 역할 |
|------|------|------|
| Reconciler | `kernel/engine/reconciler.go` | 상태 동기화 엔진 |
| YAML Loader | `kernel/loader/yaml_loader.go` | 선언적 설정 파싱 |
| Store | `kernel/store/*.go` | 상태 영속화 |
| MCP Server | `kernel/mcp/server.go` | AI 통합 인터페이스 |
| Context | `kernel/model/context.go` | 의존성 주입 |
| Registry | `kernel/model/registry.go` | 컴포넌트 등록 |

---

## 2. Reconciler 분석

### 2.1 현재 구현
```go
type Diff struct {
    ToCreate []ResourceChange
    ToUpdate []ResourceChange  // ⚠️ 미사용
    ToDelete []ResourceChange
}
```

### 2.2 문제점

#### P1: Update 감지 미구현 (Critical)
```go
// ComputeDiff에서 ToUpdate가 채워지지 않음
// 호스트 속성 변경 시 감지 불가
```
**영향**: 인프라 속성 변경(instanceType 등) 반영 불가

#### P2: Component 레벨 Reconciliation 부재 (Critical)
```go
// 현재: Host 단위만 비교
for id, host := range desiredHosts {
    if _, exists := currentHosts[id]; !exists {
        diff.ToCreate = append(...)
    }
}
// 문제: Component 추가/삭제/변경 감지 불가
```

#### P3: 실제 인프라 프로비저닝 훅 부재 (Critical)
```go
// 현재: Store에만 기록
if err := r.Store.SaveResource(instanceId, resource); err != nil {
    return result, err
}
// 문제: Terraform, SSH 등 실제 프로비저닝 연동 없음
```

#### P4: 상태값 하드코딩 (Medium)
```go
Status: "running",  // 항상 running
```

#### P5: 에러 집계 미구현 (Medium)
```go
// 첫 에러 발생 시 즉시 반환
// 부분 실패 시 어디까지 성공했는지 불명확
```

#### P6: Dry-run 미지원 (Low)
Reconciler 자체에 dry-run 모드 없음

### 2.3 개선안

```go
// 개선된 Diff 구조
type ResourceChange struct {
    Id         string
    Type       string
    RegionId   string
    HostId     string
    Action     Action  // CREATE, UPDATE, DELETE
    OldState   *ResourceState
    NewState   *ResourceState
    Reason     string  // 변경 사유
}

type Action string
const (
    ActionCreate Action = "create"
    ActionUpdate Action = "update"
    ActionDelete Action = "delete"
)

// 개선된 ReconcileResult
type ReconcileResult struct {
    Created   int
    Updated   int
    Deleted   int
    Unchanged int
    Errors    []ReconcileError  // 부분 실패 추적
    DryRun    bool
}

// 프로비저닝 훅 인터페이스
type Provisioner interface {
    CreateHost(ctx context.Context, host *model.Host) error
    UpdateHost(ctx context.Context, host *model.Host, changes []string) error
    DeleteHost(ctx context.Context, hostId string) error
}
```

---

## 3. YAML Loader 분석

### 3.1 현재 구현
```go
type FablabYaml struct {
    Model   ModelYaml
    Regions map[string]RegionYaml
}
```

### 3.2 문제점

#### P1: 제한적 스키마 (Critical)
```yaml
# 지원됨
model:
  id: my-network
regions:
  us-east-1:
    hosts:
      host1:
        components:
          - type: ziti-controller

# 미지원 (fablab 원본 기능들)
# - scope, variables, bindings
# - actions, factories
# - lifecycle stages
# - component 상세 설정
```

#### P2: 검증 부재 (High)
```go
// 컴포넌트 타입만 검증
compType, err := model.GetComponentType(config.Type)
// 미검증:
// - 필수 필드
// - 값 범위
// - 참조 무결성
```

#### P3: 스키마 버저닝 없음 (Medium)
```yaml
# 버전 정보 없음
# apiVersion: fablab/v1 같은 필드 필요
```

#### P4: 기본값 처리 부재 (Medium)
```go
// instanceType 미지정 시 빈 문자열
host := &model.Host{
    InstanceType: config.InstanceType,  // "" 가능
}
```

#### P5: Include/Reference 미지원 (Low)
```yaml
# 다른 YAML 참조 불가
# $ref: ./base-config.yaml
```

### 3.3 개선안

```go
// 확장된 스키마
type FablabYaml struct {
    APIVersion string                `yaml:"apiVersion"`  // "fablab/v1"
    Kind       string                `yaml:"kind"`        // "Model"
    Metadata   MetadataYaml          `yaml:"metadata"`
    Spec       ModelSpecYaml         `yaml:"spec"`
}

type ModelSpecYaml struct {
    Model     ModelYaml             `yaml:"model"`
    Defaults  DefaultsYaml          `yaml:"defaults"`
    Variables map[string]string     `yaml:"variables"`
    Regions   map[string]RegionYaml `yaml:"regions"`
}

type DefaultsYaml struct {
    InstanceType string `yaml:"instanceType"`
    Site         string `yaml:"site"`
}

// 검증 인터페이스
type Validator interface {
    Validate(config *FablabYaml) []ValidationError
}

type ValidationError struct {
    Path    string  // "regions.us-east-1.hosts.host1"
    Field   string  // "instanceType"
    Message string  // "required field missing"
    Severity string // "error", "warning"
}
```

---

## 4. Store 분석

### 4.1 현재 구현
```go
type ResourceState struct {
    Id       string
    Type     string
    Status   string
    Metadata map[string]string
}
```

### 4.2 문제점

#### P1: 타임스탬프 부재 (High)
```go
// 생성/수정 시간 추적 불가
// 디버깅, 감사 어려움
```

#### P2: 버전/히스토리 없음 (High)
```go
// 상태 변경 이력 추적 불가
// 롤백 불가
```

#### P3: 트랜잭션 미지원 (Medium)
```go
// 여러 리소스 원자적 업데이트 불가
// 부분 실패 시 불일치 상태 가능
```

#### P4: 배치 연산 없음 (Medium)
```go
// 대량 리소스 처리 시 비효율
for _, change := range diff.ToCreate {
    r.Store.SaveResource(...)  // N번 호출
}
```

#### P5: 쿼리/필터 기능 없음 (Low)
```go
// 특정 조건 리소스 조회 불가
// GetResources는 전체 반환만 가능
```

### 4.3 개선안

```go
// 확장된 ResourceState
type ResourceState struct {
    Id        string            `json:"id"`
    Type      string            `json:"type"`
    Status    ResourceStatus    `json:"status"`
    Metadata  map[string]string `json:"metadata"`
    CreatedAt time.Time         `json:"createdAt"`
    UpdatedAt time.Time         `json:"updatedAt"`
    Version   int64             `json:"version"`
}

type ResourceStatus string
const (
    StatusPending  ResourceStatus = "pending"
    StatusCreating ResourceStatus = "creating"
    StatusRunning  ResourceStatus = "running"
    StatusUpdating ResourceStatus = "updating"
    StatusDeleting ResourceStatus = "deleting"
    StatusDeleted  ResourceStatus = "deleted"
    StatusError    ResourceStatus = "error"
)

// 확장된 인터페이스
type ResourceStore interface {
    StateStore

    // 기본 CRUD
    GetResources(instanceId string) (map[string]ResourceState, error)
    GetResource(instanceId, resourceId string) (*ResourceState, error)
    SaveResource(instanceId string, resource ResourceState) error
    DeleteResource(instanceId, resourceId string) error

    // 배치 연산
    SaveResources(instanceId string, resources []ResourceState) error
    DeleteResources(instanceId string, resourceIds []string) error

    // 쿼리
    QueryResources(instanceId string, filter ResourceFilter) ([]ResourceState, error)

    // 트랜잭션
    BeginTx() (StoreTx, error)
}

type ResourceFilter struct {
    Types    []string
    Statuses []ResourceStatus
    Since    *time.Time
}

type StoreTx interface {
    SaveResource(instanceId string, resource ResourceState) error
    DeleteResource(instanceId, resourceId string) error
    Commit() error
    Rollback() error
}
```

---

## 5. MCP Server 분석

### 5.1 현재 구현
- 5개 Tools: list_instances, get_instance, apply_config, get_resources, create_network
- 2개 Resources: fablab://status, fablab://instances/{id}

### 5.2 문제점

#### P1: delete_instance 도구 부재 (High)
```go
// 인스턴스 삭제 불가
// create는 있으나 delete 없음
```

#### P2: JSON Marshal 에러 무시 (Medium)
```go
result, _ := json.MarshalIndent(...)  // 에러 무시
```

#### P3: 설정 경로 검증 부재 (Medium)
```go
// 경로 존재 여부, 권한 등 미검증
m, err := loader.LoadModel(configPath)
```

#### P4: URI 파싱 취약 (Medium)
```go
// 하드코딩된 접두사 제거
instanceId := request.Params.URI[len("fablab://instances/"):]
// 예외 처리 없음
```

#### P5: 인증/인가 없음 (Low - MCP 특성상)
MCP 프로토콜 자체가 로컬 통신 기반

### 5.3 개선안

```go
// 추가 도구
func (fs *FablabMCPServer) registerTools() {
    // ... 기존 도구 ...

    // delete_instance 추가
    deleteTool := mcp.NewTool("delete_instance",
        mcp.WithDescription("Delete a fablab instance and all its resources"),
        mcp.WithString("instance_id", mcp.Description("ID of the instance"), mcp.Required()),
        mcp.WithBoolean("force", mcp.Description("Force delete without confirmation")),
    )
    fs.server.AddTool(deleteTool, fs.deleteInstanceHandler)

    // validate_config 추가
    validateTool := mcp.NewTool("validate_config",
        mcp.WithDescription("Validate a YAML configuration without applying"),
        mcp.WithString("config_path", mcp.Description("Path to YAML file"), mcp.Required()),
    )
    fs.server.AddTool(validateTool, fs.validateConfigHandler)

    // diff_config 추가
    diffTool := mcp.NewTool("diff_config",
        mcp.WithDescription("Show differences between config and current state"),
        mcp.WithString("config_path", mcp.Description("Path to YAML file"), mcp.Required()),
    )
    fs.server.AddTool(diffTool, fs.diffConfigHandler)
}

// 에러 처리 개선
func marshalJSON(v interface{}) (string, error) {
    result, err := json.MarshalIndent(v, "", "  ")
    if err != nil {
        return "", fmt.Errorf("failed to marshal JSON: %w", err)
    }
    return string(result), nil
}

// URI 파싱 개선
func parseInstanceURI(uri string) (string, error) {
    const prefix = "fablab://instances/"
    if !strings.HasPrefix(uri, prefix) {
        return "", fmt.Errorf("invalid instance URI: %s", uri)
    }
    instanceId := strings.TrimPrefix(uri, prefix)
    if instanceId == "" {
        return "", fmt.Errorf("empty instance ID in URI: %s", uri)
    }
    return instanceId, nil
}
```

---

## 6. Context 분석

### 6.1 문제점

#### P1: InstanceConfig 미설정 (Medium)
```go
func NewContext(m *Model, l *Label, c *FablabConfig) *Context {
    return &Context{
        Model:  m,
        Label:  l,
        Config: c,
        // InstanceConfig 미설정
    }
}
```

#### P2: 검증 부재 (Medium)
```go
// nil 모델로 Context 생성 가능
ctx := NewContext(nil, nil, nil)
ctx.GetModel()  // nil 반환, 이후 NPE 가능
```

#### P3: Context 취소 미지원 (Low)
```go
// context.Context와 통합되지 않음
// 장시간 작업 취소 불가
```

### 6.2 개선안

```go
import "context"

type Context struct {
    ctx            context.Context
    cancel         context.CancelFunc
    Model          *Model
    Label          *Label
    Config         *FablabConfig
    InstanceConfig *InstanceConfig
}

func NewContext(m *Model, l *Label, c *FablabConfig) (*Context, error) {
    if m == nil {
        return nil, errors.New("model is required")
    }

    ctx, cancel := context.WithCancel(context.Background())
    return &Context{
        ctx:    ctx,
        cancel: cancel,
        Model:  m,
        Label:  l,
        Config: c,
    }, nil
}

func (c *Context) Done() <-chan struct{} {
    return c.ctx.Done()
}

func (c *Context) Cancel() {
    c.cancel()
}

func (c *Context) WithTimeout(d time.Duration) (*Context, context.CancelFunc) {
    ctx, cancel := context.WithTimeout(c.ctx, d)
    newCtx := *c
    newCtx.ctx = ctx
    newCtx.cancel = cancel
    return &newCtx, cancel
}
```

---

## 7. Registry 분석

### 7.1 문제점

#### P1: 등록된 타입 조회 불가 (Medium)
```go
// 어떤 컴포넌트가 등록되어 있는지 확인 불가
// YAML 작성 시 유효한 타입 알 수 없음
```

#### P2: 메타데이터 없음 (Low)
```go
// 컴포넌트 설명, 필수 설정 등 정보 없음
```

### 7.2 개선안

```go
type ComponentInfo struct {
    Name        string
    Description string
    Factory     ComponentFactory
    Schema      *ComponentSchema  // 설정 스키마
}

type ComponentSchema struct {
    RequiredFields []string
    OptionalFields []string
    Defaults       map[string]interface{}
}

var registry = make(map[string]ComponentInfo)

func RegisterComponentType(name string, factory ComponentFactory, opts ...ComponentOption) {
    info := ComponentInfo{Name: name, Factory: factory}
    for _, opt := range opts {
        opt(&info)
    }
    registry[name] = info
}

func ListComponentTypes() []string {
    types := make([]string, 0, len(registry))
    for name := range registry {
        types = append(types, name)
    }
    sort.Strings(types)
    return types
}

func GetComponentInfo(name string) (*ComponentInfo, bool) {
    info, ok := registry[name]
    return &info, ok
}
```

---

## 8. 테스트 커버리지 분석

### 8.1 현재 상태
| 모듈 | 테스트 수 | 커버리지 | 비고 |
|------|----------|---------|------|
| reconciler | 5 | ~60% | Update 테스트 없음 |
| loader | 4 | ~70% | 에러 케이스 부족 |
| store | 2 | ~50% | FileStore 테스트 부족 |
| mcp | 7 | ~65% | 에러 핸들러 미테스트 |
| context | 5 | ~80% | 양호 |
| registry | 3 | ~90% | 양호 |

### 8.2 필요 테스트
1. Reconciler Update 시나리오
2. Component 레벨 변경 감지
3. 동시성 테스트 (Store)
4. 대용량 데이터 테스트
5. 에러 복구 시나리오
6. 통합 테스트 (Reconciler + Store + MCP)

---

## 9. 개선 우선순위

### Critical (즉시 수정)
1. ~~Reconciler Update 감지 구현~~ → 신규
2. ~~Component 레벨 Reconciliation~~ → 신규
3. ~~YAML 검증 로직 추가~~ → 신규

### High (1주 내)
4. Store 타임스탬프 추가
5. MCP delete_instance 도구 추가
6. 에러 처리 강화

### Medium (2주 내)
7. Store 트랜잭션 지원
8. Context 취소 지원
9. Registry 메타데이터

### Low (향후)
10. YAML Include 지원
11. Store 쿼리 기능
12. 프로비저닝 훅 인터페이스

---

## 10. 결론

현재 구현은 **MVP(Minimum Viable Product)** 수준으로, 기본적인 선언적 인프라 관리 흐름은 작동하나 프로덕션 환경에서 사용하기에는 다음 영역의 보강이 필요하다:

1. **상태 동기화 완전성**: Update 감지 및 Component 레벨 지원
2. **데이터 무결성**: 검증, 타임스탬프, 트랜잭션
3. **운영성**: 에러 처리, 로깅, 모니터링 훅
4. **확장성**: 프로비저닝 인터페이스 추상화

이 개선을 통해 Fabops는 AI 기반 인프라 관리의 완전체로 발전할 수 있다.
