# Fablab 코드베이스 구조 및 아키텍처 분석

## 1. 프로젝트 개요

**Fablab (Fabulous Laboratory)**은 OpenZiti 네트워크를 위한 Go 기반 인프라 오케스트레이션 프레임워크입니다.
DSL이나 YAML 대신 **실제 Go 코드**로 인프라, 설정, 운영 워크플로우를 표현하는 "프로그래밍 툴킷"입니다.

### 핵심 철학
> "As Code"를 진짜 코드로 구현

- 기존 IaC 도구들의 제한적인 DSL 대신 범용 프로그래밍 언어(Go) 사용
- 복잡한 분산 시스템의 오케스트레이션을 코드로 표현
- 개발 → 배포 → 운영 → 폐기까지 전체 라이프사이클 관리

---

## 2. 디렉토리 구조

```
fablab/
├── cmd/fablab/           # CLI 진입점
│   ├── main.go           # 명령어 위임 로직
│   └── subcmd/           # Cobra 명령어 구현체들
│       ├── root.go       # 루트 명령어 정의
│       ├── up.go         # 라이프사이클 실행
│       ├── serve.go      # MCP 서버 시작
│       ├── create.go     # 인스턴스 생성
│       ├── dispose.go    # 인프라 해제
│       └── ...
├── kernel/               # 핵심 프레임워크
│   ├── model/            # 엔티티 타입, 라이프사이클, 인터페이스
│   │   ├── model.go      # Model, Region, Host 정의
│   │   ├── component.go  # Component 타입 시스템
│   │   ├── scope.go      # 변수 해석 시스템
│   │   ├── registry.go   # 컴포넌트 레지스트리 (NEW)
│   │   ├── context.go    # 컨텍스트 기반 아키텍처 (NEW)
│   │   ├── globals.go    # 전역 상태 (DEPRECATED 예정)
│   │   └── label.go      # 인스턴스 상태 관리
│   ├── lib/              # 유틸리티 및 기본 요소
│   │   ├── actions/      # 재사용 가능한 액션들
│   │   ├── runlevel/     # 단계별 구현체 (0-6)
│   │   │   ├── 0_infrastructure/  # Express 단계
│   │   │   ├── 1_configuration/   # Build 단계
│   │   │   ├── 3_distribution/    # Sync 단계
│   │   │   └── 5_operation/       # Operate 단계
│   │   └── parallel/     # 동시성 헬퍼
│   ├── libssh/           # SSH/SFTP 작업
│   ├── loader/           # YAML 모델 로딩 (NEW)
│   ├── store/            # 상태 영속화 (NEW)
│   ├── engine/           # 조정(Reconciliation) 로직 (NEW)
│   └── mcp/              # MCP 서버 (NEW)
├── resources/            # 임베디드 리소스
├── docs/                 # 문서 및 예제
└── main.go               # 라이브러리 진입점
```

---

## 3. 핵심 아키텍처: 3계층 모델

### 3.1 구조적 모델 (Structural Model)

분산 환경의 "디지털 트윈"을 Go 데이터 구조로 표현합니다.

#### 엔티티 계층 구조

```
Model (최상위)
  └── Region (지역/데이터센터)
        └── Host (서버/VM)
              └── Component (소프트웨어 컴포넌트)
```

#### 주요 구조체 (`kernel/model/model.go`)

```go
type Model struct {
    Id string
    Scope                        // 변수 스코프
    Regions         Regions      // map[string]*Region
    Factories       []Factory    // 모델 빌더
    Infrastructure  Stages       // Express 단계
    Configuration   Stages       // Build 단계
    Distribution    Stages       // Sync 단계
    Activation      Stages       // Activate 단계
    Operation       Stages       // Operate 단계
    Disposal        Stages       // Dispose 단계
    Actions         map[string]ActionBinder  // 커스텀 액션
}

type Region struct {
    Scope
    Model  *Model
    Id     string
    Hosts  Hosts  // map[string]*Host
}

type Host struct {
    Scope
    Region     *Region
    PublicIp   string
    PrivateIp  string
    Components Components  // map[string]*Component
    // SSH 연결 관리
    sshClient  *ssh.Client
}

type Component struct {
    Scope
    Host *Host
    Type ComponentType  // 실제 동작 정의
}
```

### 3.2 행위적 모델 (Behavioral Model)

구조적 모델에 대한 동작을 정의하는 인터페이스 시스템입니다.

#### ComponentType 인터페이스 계층 (`kernel/model/component.go`)

```go
// 모든 컴포넌트가 반드시 구현해야 하는 기본 인터페이스
type ComponentType interface {
    Label() string                           // 사용자 친화적 라벨
    GetVersion() string                      // 버전 정보
    Dump() any                               // 디버깅용 덤프
    IsRunning(run Run, c *Component) (bool, error)  // 실행 상태 확인
    Stop(run Run, c *Component) error        // 프로세스 중지
}

// 선택적 인터페이스들 (필요에 따라 구현)
type ServerComponent interface {             // 백그라운드 실행 가능
    Start(run Run, c *Component) error
}

type FileStagingComponent interface {        // Build 단계에서 파일 생성
    StageFiles(r Run, c *Component) error
}

type HostInitializingComponent interface {   // Sync 단계에서 호스트 설정
    InitializeHost(r Run, c *Component) error
}

type InitializingComponent interface {       // Activate 단계에서 초기화
    Init(r Run, c *Component) error
}
```

#### Factory 인터페이스 (`kernel/model/factory.go`)

```go
type Factory interface {
    Build(m *Model) error  // 모델 구조 생성/수정
}
```

#### Action 인터페이스 (`kernel/model/model.go`)

```go
type Action interface {
    Execute(run Run) error  // 커스텀 작업 실행
}

type Stage interface {
    Execute(run Run) error  // 라이프사이클 단계 실행
}
```

### 3.3 인스턴스 모델 (Instance Model)

모델의 런타임 상태를 관리합니다.

#### Label 구조체 (`kernel/model/label.go`)

```go
type Label struct {
    InstanceId string        `yaml:"id"`
    Model      string        `yaml:"model"`
    State      InstanceState `yaml:"state"`
    Bindings   Variables     `yaml:"bindings"`  // IP 주소 등 동적 바인딩
}

type InstanceState int  // Created → Expressed → Configured → Distributed → Activated → Operating → Disposed
```

---

## 4. 배포 라이프사이클 (6단계)

### 라이프사이클 흐름

```
┌─────────────────────────────────────────────────────────────────┐
│  Created → Express → Build → Sync → Activate → Operate → Dispose │
│     ↓        ↓        ↓       ↓        ↓         ↓        ↓      │
│   생성    인프라    설정    배포     가동      운영     해제     │
└─────────────────────────────────────────────────────────────────┘
```

### 각 단계 상세

| 단계 | 메서드 | 설명 | 주요 작업 |
|------|--------|------|----------|
| **Express** | `Model.Express()` | 인프라 프로비저닝 | Terraform 실행, AWS 리소스 생성 |
| **Build** | `Model.Build()` | 설정 파일 생성 | PKI 생성, 설정 템플릿 렌더링 |
| **Sync** | `Model.Sync()` | 아티팩트 배포 | rsync/sftp로 파일 전송 |
| **Activate** | `Model.Activate()` | 서비스 시작 | 프로세스 실행, 초기화 |
| **Operate** | `Model.Operate()` | 운영 작업 | 메트릭 수집, 모니터링 |
| **Dispose** | `Model.Dispose()` | 정리 | 인프라 삭제 |

### `up` 명령어 실행 흐름 (`cmd/fablab/subcmd/up.go`)

```go
func up(_ *cobra.Command, _ []string) {
    model.Bootstrap()
    ctx, _ := model.NewRun(model.GetModel(), model.GetLabel(), model.GetActiveInstanceConfig())

    ctx.GetModel().Express(ctx)   // 인프라 생성
    ctx.GetModel().Build(ctx)     // 설정 생성
    ctx.GetModel().Sync(ctx)      // 파일 배포
    ctx.GetModel().Activate(ctx)  // 서비스 시작
}
```

---

## 5. 변수 해석 시스템 (Scope)

### 변수 해석 우선순위

```
1. 커맨드라인 인자 (-V variable=value)
2. 환경 변수
3. Label 데이터 (인스턴스별)
4. Bindings (전역 설정)
5. 계층적 스코프 (부모 엔티티로 올라가며 검색)
```

### VariableResolver 체인 (`kernel/model/scope.go`)

```go
type VariableResolver interface {
    Resolve(entity Entity, name string, scoped bool) (interface{}, bool)
}

// 해석기 체인 구성
ChainedVariableResolver
  ├── CmdLineArgVariableResolver    // -V 플래그
  ├── EnvVariableResolver           // 환경 변수
  ├── MapVariableResolver (label)   // 인스턴스 상태
  ├── MapVariableResolver (bindings)// 전역 바인딩
  └── HierarchicalVariableResolver  // 엔티티 계층 탐색
```

### 변수 사용 예시

```go
// 점 표기법으로 중첩된 변수 접근
host.MustStringVariable("credentials.ssh.username")
host.GetBoolVariable("component.enabled")
host.GetVariableOr("timeout", 30)
```

---

## 6. 새로운 아키텍처 컴포넌트 (전환 중)

### 6.1 컴포넌트 레지스트리 (`kernel/model/registry.go`)

YAML 기반 설정을 위한 컴포넌트 타입 등록 시스템입니다.

```go
// 컴포넌트 팩토리 등록
func RegisterComponentType(typeName string, factory ComponentFactory)

// 이름으로 컴포넌트 타입 조회
func GetComponentType(typeName string) (ComponentType, error)
```

#### 사용 패턴

```go
// 컴포넌트 등록 (init에서)
func init() {
    RegisterComponentType("generic", func() ComponentType {
        return &GenericComponent{}
    })
}

// YAML에서 참조
components:
  - type: ziti-router  # 레지스트리에서 조회됨
```

### 6.2 Context 기반 아키텍처 (`kernel/model/context.go`)

전역 싱글톤에서 명시적 컨텍스트 전달로 전환 중입니다.

```go
type Context struct {
    Model  *Model
    Label  *Label
    Config *FablabConfig
}

// 기존 (deprecated)
model.GetModel()  // 전역 싱글톤

// 신규 (권장)
ctx.GetModel()    // 컨텍스트에서 획득
```

### 6.3 StateStore 인터페이스 (`kernel/store/interface.go`)

상태 영속화를 위한 추상화 계층입니다.

```go
type StateStore interface {
    GetStatus(instanceId string) (*model.Label, error)
    SaveStatus(instanceId string, label *model.Label) error
    ListInstances() ([]string, error)
}
```

#### 구현체: FileStore (`kernel/store/file_store.go`)

```go
type FileStore struct {
    Config *model.FablabConfig
}

// ~/.fablab/ 디렉토리에 상태 저장
```

### 6.4 Reconciler (`kernel/engine/reconciler.go`)

원하는 상태(Desired State)와 현재 상태(Current State)를 비교하고 수렴시킵니다.

```go
type Reconciler struct {
    Store store.StateStore
}

func (r *Reconciler) Reconcile(ctx *model.Context) error {
    // 1. 현재 상태 로드
    // 2. 원하는 상태와 비교 (Discovery)
    // 3. 차이점 수렴 (Convergence)
}
```

### 6.5 MCP 서버 (`kernel/mcp/server.go`)

AI 에이전트(Claude/Cursor) 통합을 위한 Model Context Protocol 서버입니다.

```go
type FablabMCPServer struct {
    server *server.MCPServer
    store  store.StateStore
}

// 리소스: fablab://status - 배포 상태 조회
// 도구: create_network - 네트워크 생성
```

#### 사용 방법

```bash
fablab serve  # MCP 서버 시작 (stdio)
```

---

## 7. CLI 명령어 구조

### 명령어 위임 메커니즘 (`cmd/fablab/main.go`)

```go
func main() {
    // 특정 명령어는 로컬 실행
    if os.Args[1] == "completion" || os.Args[1] == "clean" || os.Args[1] == "serve" {
        subcmd.Execute()
        return
    }

    // 나머지는 인스턴스별 바이너리로 위임
    instance := cfg.Instances[cfg.Default]
    cmd := exec.Command(instance.Executable, os.Args[1:]...)
    cmd.Run()
}
```

### 주요 명령어

| 명령어 | 설명 |
|--------|------|
| `fablab create <instance>` | 새 인스턴스 생성 |
| `fablab list` | 모든 인스턴스 목록 |
| `fablab use <instance>` | 기본 인스턴스 설정 |
| `fablab up` | Express → Build → Sync → Activate 실행 |
| `fablab run <action>` | 커스텀 액션 실행 |
| `fablab dispose` | 인프라 해제 |
| `fablab serve` | MCP 서버 시작 |
| `fablab ssh <host>` | 호스트에 SSH 접속 |

---

## 8. SSH 및 원격 실행 (`kernel/model/model.go`)

### Host의 원격 실행 기능

```go
// 명령어 실행
output, err := host.ExecLogged("systemctl status ziti-router")

// 타임아웃 포함 실행
output, err := host.ExecLoggedWithTimeout(30*time.Second, "apt update")

// 파일 전송
host.SendFile("/local/path", "/remote/path")

// 프로세스 관리
host.KillProcesses("-9", func(line string) bool {
    return strings.Contains(line, "ziti-router")
})
```

### SSH 연결 관리

```go
type Host struct {
    sshLock          sync.Mutex
    sshClient        *ssh.Client
    sshConfigFactory libssh.SshConfigFactory
}
```

---

## 9. 병렬 처리 (`kernel/model/model.go`)

### ForEachHost / ForEachComponent

```go
// 최대 100개 동시 실행
err := m.ForEachHost("*", 100, func(host *Host) error {
    return host.ExecLogOnlyOnError("apt update")
})

// 모든 컴포넌트에 대해 실행
err := m.ForEachComponent("*", 50, func(c *Component) error {
    return c.Type.Stop(run, c)
})
```

---

## 10. 아키텍처 전환 로드맵

### 현재 (안정)

- Go 코드로 모델 정의
- 싱글톤 `model.GetModel()`
- CLI 기반 인스턴스 관리

### 진행 중

| 컴포넌트 | 파일 | 상태 |
|----------|------|------|
| Component Registry | `kernel/model/registry.go` | 구현 완료 |
| Context 기반 전달 | `kernel/model/context.go` | 구현 완료 |
| MCP 서버 | `kernel/mcp/server.go` | 기본 구현 |
| StateStore | `kernel/store/` | 기본 구현 |

### 계획 중

| 기능 | 설명 |
|------|------|
| YAML 기반 배포 | `kernel/loader/yaml_loader.go` |
| 상태 조정 엔진 | `kernel/engine/reconciler.go` |
| GitOps 워크플로우 | 선언적 인프라 관리 |

---

## 11. 빌드 및 테스트

### 빌드

```bash
# 바이너리 빌드
go build -o fablab ./cmd/fablab
```

### 테스트

```bash
# 전체 테스트
go test ./...

# API 테스트 태그 포함 (CI와 동일)
go test ./... --tags apitests

# 특정 패키지 테스트
go test ./kernel/model -run TestScopedVariableResolver
```

### 린팅

```bash
golangci-lint run
```

---

## 12. 주요 의존성

| 패키지 | 용도 |
|--------|------|
| `github.com/spf13/cobra` | CLI 프레임워크 |
| `github.com/sirupsen/logrus` | 로깅 |
| `golang.org/x/crypto/ssh` | SSH 연결 |
| `github.com/pkg/sftp` | SFTP 파일 전송 |
| `github.com/aws/aws-sdk-go` | AWS 통합 |
| `github.com/mark3labs/mcp-go` | MCP 서버 |
| `gopkg.in/yaml.v2` | YAML 파싱 |

---

## 13. 개발 시 주의사항

### 코드 작성 시

1. **새 컴포넌트는 레지스트리 패턴 사용**
   ```go
   func init() {
       model.RegisterComponentType("my-component", func() model.ComponentType {
           return &MyComponent{}
       })
   }
   ```

2. **전역 `model.GetModel()` 사용 지양** - 컨텍스트 전달 방식 선호

3. **ComponentType 인터페이스 반드시 구현** - 미구현 시 배포 불가

### 아키텍처 이해

- `docs/` 및 루트의 `*.md` 파일들 참조
- 현재 코드와 전환 중인 코드 구분 필요
- 새 기능은 Context 기반 아키텍처로 구현

---

## 14. 파일별 역할 요약

| 파일 | 역할 |
|------|------|
| `kernel/model/model.go` | 핵심 엔티티 정의 (Model, Region, Host, Run) |
| `kernel/model/component.go` | Component 타입 시스템 및 인터페이스 |
| `kernel/model/scope.go` | 변수 해석 시스템 |
| `kernel/model/label.go` | 인스턴스 상태 영속화 |
| `kernel/model/registry.go` | 컴포넌트 타입 레지스트리 |
| `kernel/model/context.go` | Context 기반 아키텍처 |
| `kernel/model/globals.go` | 전역 상태 (deprecated 예정) |
| `kernel/store/interface.go` | StateStore 인터페이스 |
| `kernel/engine/reconciler.go` | 상태 조정 엔진 |
| `kernel/mcp/server.go` | MCP 서버 구현 |
| `cmd/fablab/subcmd/up.go` | 라이프사이클 실행 명령어 |
| `cmd/fablab/subcmd/serve.go` | MCP 서버 시작 명령어 |
