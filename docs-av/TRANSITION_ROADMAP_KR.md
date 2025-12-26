# Fablab 아키텍처 전환 우선순위 로드맵

## 전략적 접근: 점진적 전환 (Strangler Fig Pattern)

기존 시스템을 유지하면서 새 아키텍처를 점진적으로 도입합니다.
**기존 기능 손상 없이** 새 기능을 추가하는 방식입니다.

---

## Phase 0: 기반 정비 (1주)

### 목표
새 아키텍처 도입 전 기존 코드베이스 정리

### 작업 항목

| 우선순위 | 작업 | 파일 | 난이도 |
|----------|------|------|--------|
| 0.1 | 테스트 커버리지 확인 | `*_test.go` | 낮음 |
| 0.2 | 기존 ComponentType 목록화 | 외부 프로젝트 분석 | 중간 |
| 0.3 | 의존성 그래프 문서화 | `globals.go` 사용처 | 낮음 |

### 산출물
- [ ] 기존 컴포넌트 타입 목록 (ziti-controller, ziti-router 등)
- [ ] `model.GetModel()` 호출 지점 전체 목록
- [ ] 현재 테스트 커버리지 보고서

---

## Phase 1: 컴포넌트 레지스트리 완성 (2주)

### 목표
YAML 기반 배포의 **전제조건** 완성

### 왜 먼저?
- 다른 모든 새 기능의 **기반**
- 기존 코드에 **영향 없음** (추가만 함)
- 독립적으로 테스트 가능

### 작업 항목

| 우선순위 | 작업 | 파일 | 난이도 |
|----------|------|------|--------|
| 1.1 | GenericComponent 확장 | `kernel/model/generic_component.go` | 낮음 |
| 1.2 | ZitiControllerComponent 등록 | `kernel/model/ziti_controller.go` (신규) | 중간 |
| 1.3 | ZitiRouterComponent 등록 | `kernel/model/ziti_router.go` (신규) | 중간 |
| 1.4 | 레지스트리 조회 테스트 | `kernel/model/registry_test.go` (신규) | 낮음 |
| 1.5 | 컴포넌트 타입 검증 로직 | `kernel/model/registry.go` | 중간 |

### 코드 예시

```go
// kernel/model/ziti_controller.go
package model

type ZitiControllerComponent struct {
    GenericComponent
    CtrlPort    int    `yaml:"ctrlPort"`
    MgmtPort    int    `yaml:"mgmtPort"`
    // ... 기존 ZitiController 필드들
}

func (c *ZitiControllerComponent) Label() string {
    return "ziti-controller"
}

func (c *ZitiControllerComponent) Start(run Run, comp *Component) error {
    // 기존 시작 로직 이식
}

func init() {
    RegisterComponentType("ziti-controller", func() ComponentType {
        return &ZitiControllerComponent{
            GenericComponent: GenericComponent{Type: "ziti-controller"},
        }
    })
}
```

### 검증 기준
- [ ] 최소 5개 핵심 컴포넌트 타입 등록
- [ ] `GetComponentType("ziti-controller")` 성공
- [ ] 단위 테스트 통과

---

## Phase 2: YAML 로더 구현 (2주)

### 목표
YAML 파일에서 Model 객체 생성

### 의존성
- Phase 1 완료 (컴포넌트 레지스트리)

### 작업 항목

| 우선순위 | 작업 | 파일 | 난이도 |
|----------|------|------|--------|
| 2.1 | YAML 스키마 정의 | `kernel/loader/schema.go` (신규) | 중간 |
| 2.2 | Region 파싱 구현 | `kernel/loader/yaml_loader.go` | 중간 |
| 2.3 | Host 파싱 구현 | `kernel/loader/yaml_loader.go` | 중간 |
| 2.4 | Component 파싱 + 레지스트리 연동 | `kernel/loader/yaml_loader.go` | 높음 |
| 2.5 | 변수 해석 통합 | `kernel/loader/yaml_loader.go` | 높음 |
| 2.6 | 로더 테스트 | `kernel/loader/yaml_loader_test.go` (신규) | 중간 |

### YAML 스키마 예시

```yaml
# fablab.yaml
model:
  id: my-ziti-network

regions:
  us-east-1:
    site: aws
    hosts:
      controller:
        instanceType: t3.medium
        components:
          - type: ziti-controller
            ctrlPort: 6262
            mgmtPort: 443

      router-1:
        instanceType: t3.small
        components:
          - type: ziti-router
            mode: edge
```

### 구현 핵심 로직

```go
func LoadModel(path string) (*model.Model, error) {
    data, _ := os.ReadFile(path)
    var config FablabYaml
    yaml.Unmarshal(data, &config)

    m := &model.Model{
        Id:      config.Model.Id,
        Regions: make(model.Regions),
    }

    for regionId, regionYaml := range config.Regions {
        region := &model.Region{
            Id:    regionId,
            Hosts: make(model.Hosts),
        }

        for hostId, hostYaml := range regionYaml.Hosts {
            host := &model.Host{
                Id:           hostId,
                InstanceType: hostYaml.InstanceType,
                Components:   make(model.Components),
            }

            for i, compYaml := range hostYaml.Components {
                // 레지스트리에서 타입 조회
                compType, err := model.GetComponentType(compYaml.Type)
                if err != nil {
                    return nil, fmt.Errorf("unknown component type: %s", compYaml.Type)
                }

                comp := &model.Component{
                    Id:   fmt.Sprintf("%s-%d", compYaml.Type, i),
                    Type: compType,
                }
                host.Components[comp.Id] = comp
            }
            region.Hosts[hostId] = host
        }
        m.Regions[regionId] = region
    }

    return m, nil
}
```

### 검증 기준
- [ ] 예시 YAML 파일 로드 성공
- [ ] 생성된 Model의 구조 검증
- [ ] 잘못된 컴포넌트 타입에 대한 에러 처리

---

## Phase 3: CLI 통합 - 새 경로 추가 (1주)

### 목표
기존 CLI 유지하면서 **새 진입점** 추가

### 전략
- 기존: `fablab up` (Go 코드 기반)
- 신규: `fablab apply --config model.yaml` (YAML 기반)

### 작업 항목

| 우선순위 | 작업 | 파일 | 난이도 |
|----------|------|------|--------|
| 3.1 | apply 명령어 추가 | `cmd/fablab/subcmd/apply.go` (신규) | 중간 |
| 3.2 | --config 플래그 처리 | `cmd/fablab/subcmd/apply.go` | 낮음 |
| 3.3 | YAML 로더 연동 | `cmd/fablab/subcmd/apply.go` | 중간 |
| 3.4 | 기존 라이프사이클 재사용 | `cmd/fablab/subcmd/apply.go` | 낮음 |

### 코드 예시

```go
// cmd/fablab/subcmd/apply.go
var applyCmd = &cobra.Command{
    Use:   "apply",
    Short: "Apply infrastructure from YAML configuration",
    Run:   apply,
}

var configPath string

func init() {
    RootCmd.AddCommand(applyCmd)
    applyCmd.Flags().StringVarP(&configPath, "config", "c", "fablab.yaml", "Path to YAML config")
}

func apply(_ *cobra.Command, _ []string) {
    // YAML에서 Model 로드
    m, err := loader.LoadModel(configPath)
    if err != nil {
        logrus.Fatalf("failed to load config: %v", err)
    }

    // 기존 라이프사이클 재사용
    ctx, _ := model.NewRun(m, nil, nil)
    m.Express(ctx)
    m.Build(ctx)
    m.Sync(ctx)
    m.Activate(ctx)
}
```

### 검증 기준
- [ ] `fablab apply --config test.yaml` 실행 성공
- [ ] 기존 `fablab up` 영향 없음
- [ ] 에러 메시지 명확

---

## Phase 4: Context 기반 마이그레이션 (3주)

### 목표
전역 싱글톤 → 명시적 의존성 주입

### 왜 나중에?
- 가장 **파급력이 큼** (28개 파일 수정)
- Phase 1-3 없이도 기존 시스템 동작
- 실수하면 전체 시스템 **장애**

### 전략: 점진적 전환

```
1단계: 새 코드는 Context 사용
2단계: 기존 코드에 Context 전달 옵션 추가
3단계: 전역 함수를 Context 메서드로 대체
4단계: 전역 변수 제거
```

### 작업 항목

| 우선순위 | 작업 | 파일 | 난이도 |
|----------|------|------|--------|
| 4.1 | Context에 헬퍼 메서드 추가 | `kernel/model/context.go` | 낮음 |
| 4.2 | NewContext 팩토리 함수 | `kernel/model/context.go` | 낮음 |
| 4.3 | Bootstrap을 Context 반환으로 변경 | `kernel/model/bootstrap.go` | 중간 |
| 4.4 | subcmd 파일 하나씩 마이그레이션 | `cmd/fablab/subcmd/*.go` | 높음 |
| 4.5 | 전역 변수 deprecation 경고 | `kernel/model/globals.go` | 낮음 |
| 4.6 | 전역 변수 제거 | `kernel/model/globals.go` | 높음 |

### 마이그레이션 예시

```go
// Before (현재)
func up(_ *cobra.Command, _ []string) {
    model.Bootstrap()
    ctx, _ := model.NewRun(model.GetModel(), model.GetLabel(), ...)
    ctx.GetModel().Express(ctx)
}

// After (목표)
func up(_ *cobra.Command, _ []string) {
    appCtx, _ := model.BootstrapContext()  // Context 반환
    run, _ := appCtx.NewRun()
    appCtx.Model.Express(run)
}
```

### 검증 기준
- [ ] 모든 기존 테스트 통과
- [ ] `model.GetModel()` 호출 0개
- [ ] 멀티 인스턴스 동시 실행 테스트

---

## Phase 5: Reconciler 구현 (2주)

### 목표
Desired State ↔ Current State 조정

### 의존성
- Phase 4 완료 (Context 기반)

### 작업 항목

| 우선순위 | 작업 | 파일 | 난이도 |
|----------|------|------|--------|
| 5.1 | 상태 비교 로직 | `kernel/engine/reconciler.go` | 높음 |
| 5.2 | 차이점 계산 (Diff) | `kernel/engine/diff.go` (신규) | 높음 |
| 5.3 | 수렴 액션 생성 | `kernel/engine/actions.go` (신규) | 중간 |
| 5.4 | Dry-run 모드 | `kernel/engine/reconciler.go` | 중간 |
| 5.5 | 통합 테스트 | `kernel/engine/reconciler_test.go` | 높음 |

### 검증 기준
- [ ] 변경 없을 때 no-op
- [ ] 호스트 추가 감지 및 생성
- [ ] 컴포넌트 변경 감지 및 재시작
- [ ] Dry-run 출력 명확

---

## Phase 6: MCP 서버 실제 연동 (2주)

### 목표
AI 에이전트를 통한 실제 인프라 제어

### 의존성
- Phase 5 완료 (Reconciler)

### 작업 항목

| 우선순위 | 작업 | 파일 | 난이도 |
|----------|------|------|--------|
| 6.1 | create_network 실제 구현 | `kernel/mcp/server.go` | 중간 |
| 6.2 | scale 도구 추가 | `kernel/mcp/server.go` | 중간 |
| 6.3 | logs 리소스 추가 | `kernel/mcp/server.go` | 중간 |
| 6.4 | 상태 조회 상세화 | `kernel/mcp/server.go` | 낮음 |
| 6.5 | Claude Desktop 통합 테스트 | 수동 | 낮음 |

### 검증 기준
- [ ] Claude에서 "네트워크 생성해줘" → 실제 생성
- [ ] "현재 상태 알려줘" → 상세 정보 반환
- [ ] "라우터 2개로 늘려줘" → scale 동작

---

## 전체 타임라인

```
Phase 0: ████ (1주)
Phase 1: ████████ (2주)
Phase 2: ████████ (2주)
Phase 3: ████ (1주)
Phase 4: ████████████ (3주)
Phase 5: ████████ (2주)
Phase 6: ████████ (2주)
─────────────────────────────
총 소요: 약 13주 (3개월)
```

---

## 우선순위 결정 기준

### 높음 (Phase 1-3)
- 기존 시스템에 영향 없음
- 독립적으로 테스트 가능
- 즉시 가치 제공 (YAML 배포)

### 중간 (Phase 4-5)
- 기존 시스템 수정 필요
- 충분한 테스트 필수
- 장기적 유지보수성 향상

### 낮음 (Phase 6)
- 다른 Phase 완료 후 가능
- "있으면 좋은" 기능
- 데모/PoC 용도

---

## 리스크 관리

| 리스크 | 확률 | 영향 | 대응 |
|--------|------|------|------|
| Phase 4에서 기존 기능 손상 | 높음 | 치명적 | 철저한 테스트, 점진적 롤아웃 |
| 컴포넌트 레지스트리 누락 | 중간 | 높음 | 외부 프로젝트 분석 철저히 |
| YAML 스키마 변경 빈번 | 중간 | 중간 | 버전 관리, 마이그레이션 도구 |
| MCP 스펙 변경 | 낮음 | 낮음 | mcp-go 라이브러리 업데이트 |

---

## 권장 사항

### 즉시 시작 가능
1. **Phase 0**: 현황 파악 (1주)
2. **Phase 1**: 레지스트리 구축 (2주)

### 결정 필요
- Phase 4 범위: 전체 마이그레이션 vs 신규 코드만?
- YAML 스키마: 사용자 피드백 필요
- MCP 기능 범위: 어디까지 지원?

### 중단 고려 시점
- Phase 1-3 완료 후 재평가
- YAML 배포가 동작하면 Phase 4-6은 선택적

---

*작성일: 2024년*
*예상 총 소요: 13주 (풀타임 1인 기준)*
*실제 소요는 팀 규모, 우선순위 변경에 따라 달라질 수 있음*
