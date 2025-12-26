# Fablab 아키텍처 전환 현실 평가

## 결론: 전환은 시작도 안 됐습니다

새로 추가된 파일들은 대부분 **껍데기(scaffold)**입니다. 실제로 동작하는 코드가 아닙니다.

---

## 1. 숫자로 보는 현실

| 항목 | 수치 | 의미 |
|------|------|------|
| `model.GetModel()` 호출 | **47회** (28개 파일) | 전역 싱글톤 여전히 핵심 |
| `ctx.GetModel()` 호출 | 32회 | **착각 금지**: 새 Context가 아니라 기존 `Run` 인터페이스 |
| 등록된 컴포넌트 타입 | **1개** (`generic`) | 레지스트리 시스템 실제 사용 無 |

---

## 2. 새 컴포넌트별 실태

### `kernel/model/context.go` - 사용되지 않음

```go
type Context struct {
    Model  *Model
    Label  *Label
    Config *FablabConfig
}
```

- 24줄짜리 파일
- **어디에서도 import되지 않음**
- `cmd/fablab/subcmd/` 파일들은 여전히 `model.NewRun()` + 전역 `GetModel()` 사용

### `kernel/model/registry.go` - 껍데기

```go
var registry = make(map[string]ComponentFactory)
```

- 등록된 컴포넌트: `generic` **1개뿐**
- 실제 Ziti 컴포넌트(controller, router 등)는 **미등록**
- YAML 로더가 이걸 사용할 수 없음

### `kernel/loader/yaml_loader.go` - 미완성

```go
func LoadModel(path string) (*model.Model, error) {
    // ...yaml 파싱...
    m := &model.Model{}
    // Logic to populate m.Regions, m.Hosts, etc.
    // For now, this is a skeleton implementation.
    return m, nil  // 빈 모델 반환!
}
```

- **항상 빈 Model 반환**
- 주석에 "skeleton implementation" 명시
- YAML → Model 변환 로직 **전무**

### `kernel/engine/reconciler.go` - 미완성

```go
func (r *Reconciler) Reconcile(ctx *model.Context) error {
    // 1. Load Current State
    currentState, err := r.Store.GetStatus(instanceId)
    // ...로그만 찍음...

    // 2. Discovery / Compare
    // This would involve iterating...  ← 주석만 있음

    // 3. Convergence
    // This would trigger...  ← 주석만 있음

    return nil  // 아무것도 안 함
}
```

- **조정 로직 없음** - 주석만 있음
- 상태 비교 코드 없음
- 수렴(convergence) 코드 없음

### `kernel/mcp/server.go` - 데모 수준

```go
func (fs *FablabMCPServer) createNetworkHandler(...) {
    return mcp.NewToolResultText(
        fmt.Sprintf("Network '%s' (type: %s) created (simulation).", name, modelType)
    ), nil  // 실제로 아무것도 안 함!
}
```

- `create_network`: **시뮬레이션만** - 실제 생성 안 함
- `fablab://status`: 인스턴스 목록 조회만 가능
- 실질적인 인프라 제어 **불가**

---

## 3. 핵심 문제점

### A. 전역 싱글톤 제거 안 됨

`kernel/model/globals.go`:
```go
var model *Model
var label *Label

func GetModel() *Model {
    return model
}
```

- **여전히 존재하고 핵심적으로 사용됨**
- 모든 `subcmd/` 파일이 이것에 의존
- Context 기반 전환? 시작도 안 함

### B. 실제 마이그레이션 경로 부재

- 기존 코드 수정 없음
- 새 아키텍처로 점진적 전환 계획 없음
- 새 파일들이 기존 코드와 **연결되지 않음**

### C. 컴포넌트 레지스트리 무용지물

```
등록된 타입: generic (1개)
필요한 타입: ziti-controller, ziti-router, ziti-tunnel, etc. (수십 개)
```

- 기존 컴포넌트들이 레지스트리에 등록 안 됨
- YAML 기반 배포 **불가능**

---

## 4. 문서 vs 현실

| IMPLEMENTATION_PLAN.md 주장 | 실제 상태 |
|---------------------------|----------|
| "Context로 GetModel() 대체" | Context 파일 있으나 **미사용** |
| "Registry로 런타임 바인딩" | 컴포넌트 **1개** 등록 |
| "YAML로 선언적 배포" | 로더가 **빈 모델 반환** |
| "Reconciler로 상태 조정" | **주석만 있음** |
| "MCP로 AI 에이전트 통합" | 시뮬레이션 응답만 |

---

## 5. 정량적 평가

| 영역 | 진행률 | 상태 |
|------|--------|------|
| 인터페이스/구조체 정의 | 100% | ✅ 완료 |
| 컴포넌트 레지스트리 구현 | 5% | ⚠️ 껍데기 |
| YAML 로더 구현 | 10% | ⚠️ 껍데기 |
| Context 기반 마이그레이션 | 0% | ❌ 미착수 |
| Reconciler 구현 | 5% | ⚠️ 껍데기 |
| MCP 서버 실제 연동 | 10% | ⚠️ 데모 수준 |
| 기존 코드 수정 | 0% | ❌ 미착수 |

**전체 진행률: 약 5-10%**

---

## 6. 작동 현황

### 작동하는 것
- 기존 아키텍처 (전역 싱글톤 기반)
- 기존 CLI 명령어 전체
- 기존 라이프사이클 (Express → Activate)

### 작동하지 않는 것
- 새로 추가된 모든 "아키텍처 전환" 코드
- YAML 기반 배포
- Context 기반 의존성 주입
- 상태 조정(Reconciliation)
- MCP를 통한 실제 인프라 제어

---

## 7. 근본적 질문

이 전환이 정말 필요한가?

### 전환의 목적 (IMPLEMENTATION_PLAN.md)
1. YAML 설정 기반 배포 (GitOps)
2. 멀티테넌시 지원 (전역 상태 제거)
3. AI 에이전트 통합 (MCP)
4. 상태 기반 조정 (Reconciler)

### 현실적 고려사항
- 기존 아키텍처가 **잘 동작함**
- 전환 비용 대비 이점이 불명확
- 사용자가 Go 코드 작성을 **원하는** 케이스 존재
- YAML만으로 복잡한 오케스트레이션 표현 한계

---

*평가 일자: 2024년*
*평가 기준: 실제 코드 분석*
