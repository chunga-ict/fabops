# Fablab 주요 이슈 조사 및 의견

## 1. K8s 종속성 (K8s Dependency)
*   **현황**: `k8s.io/client-go` 등이 포함되어 있으나, 모든 사용자가 K8s를 사용하는 것은 아닙니다.
*   **제안**: **"Optional Dependency" 또는 "Adapter Pattern" 적용**
    *   핵심 `kernel/model`을 K8s Agnostic하게 유지해야 합니다.
    *   K8s 관련 코드는 `kernel/k8s_provider` 등으로 격리하고, 인터페이스를 통해서만 접근하도록 설계하여 Fablab이 "K8s 전용 도구"로 전락하는 것을 방지해야 합니다.

## 2. Fablab 프레임워크의 Go 바이너리화
*   **현황**: 현재는 라이브러리 형태이나, 계획안은 단일 실행 파일 배포를 제안합니다.
*   **제안**: **"하이브리드 배포" 전략**
    *   표준화된 YAML 배포를 위한 **공용 바이너리(Official Binary)**를 제공합니다.
    *   동시에, 커스텀 로직이 필요한 사용자를 위해 라이브러리(`github.com/openziti/fablab`) 사용 방식도 계속 지원하여 "도구이자 프레임워크"로서의 정체성을 유지합니다.

## 3. 인스턴스 저장소 연속성 (Instance Store Persistence)
*   **현황**: 로컬 파일시스템에 저장하므로 클라우드/컨테이너 환경에서 상태 유실 위험이 있습니다.
*   **제안**: **"State Backend" 추상화**
    *   인스턴스 상태 저장소를 인터페이스(`InstanceStore`)로 추상화합니다.
    *   **Local Mode**: 파일시스템 사용 / **Cluster Mode**: K8s Secret, ConfigMap, PV 등 사용.
    *   CLI에서 로컬과 원격 인스턴스를 통합 조회할 수 있어야 합니다.

## 4. 확장/축소 및 Reconciliation
*   **현황**: `Up` 단계에서 단순 생성만 수행하며, 상태 변경(Scale Out/In) 로직이 부족합니다.
*   **제안**: **"Reconciler(조정자)" 로직 강화**
    *   단순 생성을 넘어 **`Diff(Current, Desired)`** 알고리즘을 Kernel에 구현해야 합니다.
    *   "현재 3개 -> 목표 5개"일 때 "2개 추가"를 계산하는 로직이 Kernel 레벨에 있어야 CLI(`apply`)와 Operator 모두에서 동작할 수 있습니다.

## 5. 컴포넌트 인터페이스 구현
*   **현황**: Go 인터페이스 구현체로 컴포넌트를 정의하므로, 텍스트 기반인 YAML에서 이를 직접 표현하기 어렵습니다.
*   **제안**: **"Type Registry" (레지스트리) 패턴 도입**
    *   `map[string]ComponentType` 형태의 레지스트리를 통해 YAML 문자열(`type: ziti-controller`)을 실제 Go 구현체로 매핑합니다.
    *   유연성 확보를 위해 **"Generic Shell/Script Component"**를 제공하여 YAML 모드에서도 스크립트 실행이 가능하도록 합니다.

## 6. CLI 구조 유지 여부
*   **현황**: `create`, `up`, `ssh` 등 사용자 친화적 명령어를 제공합니다.
*   **제안**: **"CLI 구조 유지 및 확장" (강력 권장)**
    *   `kubectl`은 너무 low-level이므로, Fablab 모델 관점의 High-Level UX를 제공하는 CLI는 필수적입니다.
    *   CLI가 로컬/원격 환경을 투명하게 중개하는 **"Smart Client"** 역할을 수행해야 합니다 (`--local`, `--k8s` 플래그 활용).

## 요약
Fablab은 "Embedded Framework"에서 "Managed Platform"으로 진화해야 합니다. Core Kernel을 순수하게 유지하면서(Reconciliation 포함), Go Binary와 YAML 모드를 모두 지원하는 유연한 아키텍처가 필요합니다.
