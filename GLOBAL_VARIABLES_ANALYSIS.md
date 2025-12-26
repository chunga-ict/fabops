# Fablab 전역 변수(Global Variables) 및 역할 분석

Fablab 코드베이스(`kernel`, `cmd`) 내에서 시스템의 데이터와 상태를 관리하는 주요 전역 변수들을 조사하고, 그 역할과 아키텍처적 의미를 분석했습니다.

## 1. 핵심 전역 변수 (`kernel/model`)
Fablab의 시스템 상태를 저장하는 가장 중요한 변수들로, `kernel/model/globals.go` 등에 정의되어 있습니다.

| 변수명 | 타입 | 정의 위치 | 역할 및 설명 |
| :--- | :--- | :--- | :--- |
| **`model`** | `*Model` | `kernel/model/globals.go` | **시스템의 심장.** 현재 실행 중인 인프라 모델의 전체 구조(Region, Host, Component)와 로직을 담고 있는 **싱글톤(Singleton)**입니다. `main.go`에서 초기화되며, 애플리케이션 수명 동안 변경되지 않습니다. |
| **`label`** | `*Label` | `kernel/model/globals.go` | **런타임 상태 저장소.** 현재 배포된 인스턴스의 상태(State), IP 주소, 메타데이터 등을 저장합니다. 로컬 디스크(`label.json`)에 영속화되어, CLI 실행 간에 상태를 유지하는 역할을 합니다. |
| **`config`** | `*FablabConfig` | `kernel/model/instances.go` | **사용자 환경 설정.** `~/.fablab/config.yml` 파일 내용을 메모리에 로드한 것입니다. 로컬에 존재하는 여러 모델 인스턴스들의 목록과 기본 모드(Default) 설정을 담고 있습니다. |
| **`instanceConfig`** | `*InstanceConfig` | `kernel/model/instances.go` | **현재 타겟 설정.** 여러 인스턴스 중, 현재 명령어가 대상으로 하고 있는 특정 인스턴스(예: `test-network`)의 경로와 실행 정보를 담고 있습니다. |
| **`CliInstanceId`** | `string` | `kernel/model/instances.go` | **CLI 오버라이드.** 사용자가 명령어 실행 시 플래그로 전달한 인스턴스 ID를 임시 저장합니다. (예: `fablab up --instance=prod` 실행 시 "prod" 저장) |
| **`bindings`** | `Variables` | `kernel/model/globals.go` | **변수 맵.** 모델 전반에서 사용되는 환경 변수, 설정값 등의 Key-Value 저장소입니다. |

## 2. CLI 관련 전역 변수 (`cmd/fablab`)
사용자 입력을 처리하고 명령어 흐름을 제어하는 `Cobra` 라이브러리 관련 변수들입니다.

| 변수명 | 타입 | 정의 위치 | 역할 및 설명 |
| :--- | :--- | :--- | :--- |
| **`RootCmd`** | `*cobra.Command` | `cmd/fablab/subcmd/root.go` | `fablab` 명령어의 최상위 진입점입니다. 모든 하위 명령어(`up`, `ssh` 등)는 여기에 등록됩니다. |
| **`upCmd`, `sshCmd`...** | `*cobra.Command` | `cmd/fablab/subcmd/...` | 개별 하위 명령어 객체들입니다. |
| **`verbose`** | `bool` | `cmd/fablab/subcmd/root.go` | 상세 로그 출력 여부를 제어하는 플래그입니다. |

## 3. 라이브러리 레벨 전역 변수 (`kernel/lib`)
특정 기능을 수행하기 위한 싱글톤 객체들입니다.

| 변수명 | 타입 | 정의 위치 | 역할 및 설명 |
| :--- | :--- | :--- | :--- |
| **`KeyManager`** | `awsKeyManager` | `.../aws_ssh_key.go` | AWS SSH 키 쌍(Key Pair)을 생성하고 로컬 파일과 동기화하는 역할을 전담하는 싱글톤 컴포넌트입니다. |

## 4. 아키텍처 분석 및 시사점

### A. 현재 구조 (CLI 중심)
*   **특징**: `model`과 `label`이 전역 싱글톤으로 설계되어 있습니다.
*   **장점**: 한 번 실행하고 종료되는 **CLI 도구(Ephemeral Process)**로서는 구현이 단순하고 직관적입니다. "현재 실행 중인 모델은 하나다"라는 가정이 명확합니다.

### B. 미래 구조 (Operator/Service 전환 시 문제점)
앞서 논의한 "상시 서비스" 및 "설정 파일 기반 배포"로 전환 시, 현재의 전역 변수 구조는 **기술적 부채(Technical Debt)**가 됩니다.

1.  **동시성 문제 (Concurrency)**: `model` 변수가 하나뿐이므로, 하나의 프로세스 내에서 **여러 네트워크(Multi-Tenancy)**를 동시에 관리할 수 없습니다. (예: Network A와 Network B를 동시에 모니터링 불가)
2.  **상태 격리 (Isolation)**: `label`이 전역으로 선언되어 있어, 여러 인스턴스의 상태를 메모리에 동시에 로드하고 비교하는 작업(Reconciliation)을 구현하기 어렵습니다.

### C. 개선 제안 (Refactoring)
아키텍처 전환을 위해서는 **전역 변수 제거(Deprecation)** 및 **Context 기반 설계**로의 리팩토링이 필요합니다.

```go
// AS-IS: 전역 변수 접근
func DoSomething() {
    id := model.GetId() // Global access
}

// TO-BE: Context 주입
type ReconcileContext struct {
    Model *Model
    Label *Label
}

func DoSomething(ctx *ReconcileContext) {
    id := ctx.Model.GetId() // Instance specific access
}
```

**결론적으로, 현재의 전역 변수 구조는 1회성 CLI 도구에는 적합하나, 향후 목표인 '상시 운영 서비스'나 'K8s Operator'로 발전하기 위해서는 반드시 구조체/Context 기반으로 리팩토링되어야 합니다.**
