# Phase 0: 기반 정비 완료 보고서

## 0.1 테스트 커버리지 현황

### 테스트 실행 결과
```
go test ./... -cover
```

| 패키지 | 커버리지 | 상태 |
|--------|----------|------|
| `kernel/model` | **33.8%** | ⚠️ 부분 |
| `kernel/lib` | **33.3%** | ⚠️ 부분 |
| `kernel/lib/parallel` | **25.3%** | ⚠️ 부분 |
| 그 외 전체 | **0.0%** | ❌ 없음 |

### 기존 테스트 파일 (7개)
```
kernel/lib/parallel/parallel_test.go
kernel/lib/sar_test.go
kernel/model/bindings_test.go
kernel/model/model_test.go
kernel/model/scale_factory_test.go
kernel/model/scope_test.go
kernel/model/selector_test.go
```

### 테스트 실행 확인
```bash
$ go test ./kernel/model/... -v
=== RUN   TestVariableResolvingModelDefaults     --- PASS
=== RUN   TestBindBindingsRequiredToModel        --- PASS
=== RUN   TestGetVariable                        --- PASS
=== RUN   Test_Templating                        --- PASS
=== RUN   TestVariablesPut                       --- PASS
=== RUN   TestVariableResolver                   --- PASS
=== RUN   TestVariableResolverBindingsOverride   --- PASS
...
```

**결론**: 기존 테스트 모두 통과. 핵심 model 패키지만 부분 커버리지.

---

## 0.2 ComponentType 사용처 분석

### 이 저장소 내 구현체

| 타입 | 파일 | 상태 |
|------|------|------|
| `GenericComponent` | `kernel/model/generic_component.go` | ✅ 유일한 구현 |

### 레지스트리 등록 현황
```go
// kernel/model/generic_component.go
func init() {
    RegisterComponentType("generic", func() ComponentType {
        return &GenericComponent{}
    })
}
```

**등록된 타입: 1개 (`generic`)**

### ComponentType 인터페이스 사용 패턴

```go
// 필수 메서드
type ComponentType interface {
    Label() string
    GetVersion() string
    Dump() any
    IsRunning(run Run, c *Component) (bool, error)
    Stop(run Run, c *Component) error
}

// 선택적 확장
ServerComponent       // Start()
FileStagingComponent  // StageFiles()
HostInitializingComponent // InitializeHost()
InitializingComponent // Init()
```

### 핵심 발견

**fablab은 프레임워크**입니다. 실제 ComponentType 구현체(ziti-controller, ziti-router 등)는 **외부 프로젝트**에서 정의됩니다.

```
fablab (프레임워크)
  └── ComponentType 인터페이스 정의

외부 프로젝트 (예: ziti-smoketest)
  └── ZitiController implements ComponentType
  └── ZitiRouter implements ComponentType
```

**Phase 1 시사점**: 레지스트리에 등록할 표준 컴포넌트를 fablab 내부에 추가하거나, 외부 프로젝트가 등록하도록 문서화 필요.

---

## 0.3 globals.go 의존성 그래프

### 전역 변수 목록
```go
// kernel/model/globals.go
var model *Model
var label *Label
var bindings Variables
var bootstrapExtensions []BootstrapExtension
var config *FablabConfig
var instanceConfig *InstanceConfig
```

### 전역 함수 호출 통계

| 함수 | 호출 횟수 | 파일 수 |
|------|----------|---------|
| `GetModel()` | **23회** | 17개 |
| `GetLabel()` | **14회** | 13개 |
| `GetConfig()` | **5회** | 5개 |
| `GetActiveInstanceConfig()` | **14회** | 13개 |

### 파일별 의존성 맵

#### 높음 (3개 이상 호출)
| 파일 | GetModel | GetLabel | GetConfig | GetActiveInstanceConfig |
|------|----------|----------|-----------|------------------------|
| `subcmd/list.go` | 4 | - | 1 | - |
| `subcmd/create.go` | 4 | - | - | - |
| `subcmd/sync.go` | 3 | 3 | - | 3 |

#### 중간 (표준 패턴)
대부분의 subcmd 파일이 다음 패턴 사용:
```go
func someCommand(_ *cobra.Command, _ []string) {
    model.Bootstrap()
    ctx, _ := model.NewRun(
        model.GetModel(),           // 1
        model.GetLabel(),           // 2
        model.GetActiveInstanceConfig(),  // 3
    )
    // ...
}
```

### 의존성 다이어그램

```
┌─────────────────────────────────────────────────────────┐
│                    globals.go                           │
│  var model, label, bindings, config, instanceConfig     │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                 cmd/fablab/subcmd/                      │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐       │
│  │  up.go  │ │sync.go  │ │ run.go  │ │ ...     │       │
│  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘       │
│       │           │           │           │             │
│       └───────────┴───────────┴───────────┘             │
│                       │                                  │
│              model.NewRun(                              │
│                GetModel(),                              │
│                GetLabel(),                              │
│                GetActiveInstanceConfig()                │
│              )                                          │
└─────────────────────────────────────────────────────────┘
```

### 마이그레이션 영향도

| 영역 | 파일 수 | 수정 복잡도 |
|------|---------|------------|
| `cmd/fablab/subcmd/` | **17개** | 높음 |
| `cmd/fablab/main.go` | 1개 | 중간 |
| `kernel/model/` (내부) | 3개 | 높음 |

**총 수정 대상: 약 21개 파일**

---

## Phase 0 완료 체크리스트

- [x] 테스트 실행 확인 (모두 통과)
- [x] 테스트 커버리지 측정 (model: 33.8%)
- [x] ComponentType 구현체 파악 (1개: generic)
- [x] ComponentType 사용 패턴 분석
- [x] 전역 함수 호출 횟수 집계
- [x] 파일별 의존성 매핑
- [x] 마이그레이션 영향도 평가

---

## Phase 1 진행 권장사항

1. **레지스트리 테스트 먼저 작성**
   - `kernel/model/registry_test.go` 생성
   - 등록/조회 로직 검증

2. **GenericComponent 확장**
   - 설정 로딩 기능 추가
   - YAML 필드 매핑 지원

3. **외부 프로젝트 호환성 유지**
   - 기존 Go 코드 기반 사용 방식 유지
   - 레지스트리는 YAML 배포용 **추가** 옵션

---

*완료일: 2024년*
*다음 단계: Phase 1 - 컴포넌트 레지스트리 완성*
