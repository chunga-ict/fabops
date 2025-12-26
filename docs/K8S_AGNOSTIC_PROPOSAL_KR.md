# K8s 종속성 없는 Fablab 구현 제안

## 1. 기본 철학: "Provider Agnostic Architecture"
Kubernetes(K8s)를 "1급 시민(First-class Citizen)"으로 대우하되, 유일한 선택지가 되어서는 안 됩니다. Fablab의 Core Kernel은 인프라가 어디에 배포되는지 몰라야 합니다.

## 2. 아키텍처 제안

### A. Core Kernel (인프라 중립 영역)
*   **역할**: 순수한 논리적 모델링, 라이프사이클 관리, 상태 추적.
*   **구성**:
    *   `model.Model`: 인프라 논리적 구조 (Region, Host, Component).
    *   `InstanceStore Interface`: 상태 저장소 추상화 (파일, DB, API 등).
    *   `Runtime Interface`: 실제 명령을 수행하는 실행기 추상화.

### B. Runtime Providers (플러그인 형태)
Core Kernel이 정의한 인터페이스를 구현하는 구체적인 실행 모듈입니다.

1.  **Local Runtime (Default)**
    *   **Backend**: 로컬 OS 프로세스, Docker 데몬.
    *   **특징**: `os/exec`를 사용해 프로세스 실행. 현재 Fablab 동작 방식과 유사.
    *   **용도**: 개발, 테스트, 베어메탈 배포.

2.  **Kubernetes Runtime (Optional)**
    *   **Backend**: K8s API Server.
    *   **특징**: `client-go`를 사용하여 Pod/Deployment 생성.
    *   **용도**: 프로덕션 배포, 대규모 시뮬레이션.

3.  **SSH/Remote Runtime**
    *   **Backend**: 원격 서버 (AWS EC2, VM 등).
    *   **특징**: `golang.org/x/crypto/ssh`를 사용하여 원격 명령 실행. 현재 Fablab의 강점 중 하나.

### C. 인터페이스 기반 설계 (예시)

```go
// Kernel은 이 인터페이스만 알면 됩니다.
type ProcessRunner interface {
    Run(cmd string, args ...string) error
}

// 구현체 1: 로컬 실행
type LocalRunner struct {}
func (r *LocalRunner) Run(cmd string, args ...string) error {
    return exec.Command(cmd, args...).Run()
}

// 구현체 2: K8s 실행 (K8s 라이브러리가 여기서만 import됨)
type K8sRunner struct { Client kubernetes.Interface }
func (r *K8sRunner) Run(cmd string, args ...string) error {
    // Pod에 exec 요청 전송
}
```

## 3. 구현 전략: "Build Tags" 활용
K8s 라이브러리(`client-go` 50MB+)가 무거운 경우, Go의 **Build Tags** 기능을 사용하여 바이너리를 분리할 수 있습니다.

*   `fablab-lite`: K8s 의존성 제거. 로컬/SSH만 지원. 가볍고 빠름.
*   `fablab-full`: K8s 포함 모든 기능 지원.

```go
// +build k8s
package runtime
import "k8s.io/client-go/..." 
```

## 4. 결론
K8s 환경을 배제하고 구현하려면 **"인프라 추상화 계층(Interface Layer)"**을 명확히 하고, K8s 관련 코드를 **"플러그인/구현체"** 레벨로 격리해야 합니다. 이를 통해 Fablab은 로컬 프로세스부터 클라우드 클러스터까지 아우르는 진정한 범용 도구가 될 수 있습니다.
