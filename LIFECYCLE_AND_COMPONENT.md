# Fablab 배포 라이프사이클과 컴포넌트 인터페이스의 미래

사용자께서 지적하신 대로, Fablab에서 **"컴포넌트 인터페이스가 구현되지 않으면 배포가 되지 않는다"**는 원칙은 시스템의 무결성을 보장하는 핵심 제약 조건입니다. 아키텍처가 발전(설정 파일 기반, MCP 등)하더라도 이 철학이 어떻게 유지되고 진화하는지 분석했습니다.

## 1. 결론부터 말씀드리면: "유지될 뿐만 아니라, 더 표준화됩니다."

*   **배포 라이프사이클 (4-Stage)**: `Express` -> `Build` -> `Sync` -> `Activate` 로 이어지는 흐름은 물리적/논리적 인프라 배포의 본질이므로 **그대로 유지**됩니다.
*   **컴포넌트 인터페이스**: 여전히 필수입니다. 다만, "사용자가 매번 코딩하는 방식"에서 **"미리 구현된 표준 구현체를 선택하는 방식"**으로 바뀔 뿐입니다.

---

## 2. 상세 분석: 무엇이 바뀌고 무엇이 그대로인가?

### A. Lifecycle (배포 단계) - [유지]
아키텍처가 YAML 기반이나 상시 서비스로 바뀌어도, 소프트웨어를 배포하는 순서는 변하지 않습니다.

1.  **Express (인프라 마련)**: VM을 만들고 IP를 할당해야 합니다. (Terraform/AWS)
2.  **Build (준비)**: 설정 파일을 만들고 인증서를 구워야 합니다.
3.  **Sync (전송)**: 바이너리와 설정 파일을 서버로 보내야 합니다.
4.  **Activate (가동)**: `systemd`나 프로세스를 켜야 합니다.

**변화점**: 과거엔 `main.go`가 이 함수들을 직접 호출했다면, 미래엔 **Operator/Reconciler**가 이 단계들을 순차적으로 실행합니다.

### B. 컴포넌트 인터페이스 구현 - [진화: Static -> Registry]

가장 큰 변화가 일어나는 지점입니다.

#### AS-IS: "컴파일 타임 바인딩" (현재)
사용자가 `main.go`를 짤 때, 직접 구조체를 정의하고 인터페이스 메서드(`Execute`)를 구현해야 합니다.
```go
// 사용자가 직접 코딩
type MyRouter struct {}
func (r *MyRouter) Activate(ctx) error { ... } 

// 모델에 등록
Component: &MyRouter{} 
```
*   **제약**: 코드를 안 짜면 컴파일이 안 되므로 배포 불가. (**Strong Type Check**)

#### TO-BE: "런타임 레지스트리 바인딩" (미래)
사용자는 YAML 파일에 텍스트만 적습니다. Fablab 커널은 미리 구현해둔 **표준 컴포넌트 라이브러리(Registry)**에서 해당하는 구현체를 찾아서 연결합니다.

```yaml
# 사용자는 설정만 함
components:
  - id: router-1
    type: ziti-router  # <--- 이 문자열이 핵심
```

**내부 동작 (Kernel)**:
1.  YAML 파서가 `type: ziti-router`를 읽습니다.
2.  **Component Registry**를 조회합니다.
    *   `if impl, exist := Registry["ziti-router"]; exist { ... }`
3.  **[중요]** 만약 레지스트리에 `ziti-router`라는 이름의 구현체가 등록되어 있지 않다면?
    *   **"Unknown Component Type" 에러를 내며 배포를 거부합니다.**
    *   즉, **"구현체가 없으면 배포되지 않음"이라는 원칙은 런타임 에러로 여전히 강력하게 유지됩니다.**

## 3. 구현 전략: Type Registry 패턴

이 구조를 만들기 위해 Fablab 커널에 다음과 같은 **레지스트리 시스템**이 추가되어야 합니다.

```go
// kernel/model/registry.go

// 1. 컴포넌트 생성자 타입 정의
type ComponentFactory func() Component

// 2. 전역 레지스트리 맵
var componentRegistry = map[string]ComponentFactory{}

// 3. 등록 함수 (각 컴포넌트의 init()에서 호출)
func RegisterComponent(name string, factory ComponentFactory) {
    componentRegistry[name] = factory
}

// 4. 조회 함수 (YAML 로더가 호출)
func LoadComponent(name string) (Component, error) {
    if factory, ok := componentRegistry[name]; ok {
        return factory(), nil // 인터페이스 구현체 반환
    }
    return nil, fmt.Errorf("배포 불가: '%s' 타입에 대한 인터페이스 구현을 찾을 수 없습니다.", name)
}
```

## 4. 요약

1.  **Lifecycle은 Fablab의 척추**입니다. 절대 바뀌지 않습니다.
2.  **인터페이스 구현은 여전히 필수**입니다. 다만 사용자가 매번 코딩하는 대신, 개발팀이 미리 잘 만들어둔 **"표준 컴포넌트(Standard Components)"**를 YAML에서 골라 쓰는 형태로 바뀝니다.
3.  **커스텀 컴포넌트**가 필요한 경우, 여전히 Go 코드로 플러그인을 작성하여 레지스트리에 등록해야 합니다. (Go Plugin이나 별도 빌드 필요)

따라서 우려하시는 "구현 안 된 컴포넌트가 배포되는 일"은 발생하지 않습니다. 시스템이 사전에 차단하기 때문입니다.
