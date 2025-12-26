# Fablab 문서 자료 정리 및 요약

`docs/` 디렉토리에 포함된 주요 문서들의 내용을 분석하고 정리한 내용입니다. 이 문서들은 Fablab의 현재 이슈 진단, 개선 제안, 향후 아키텍처 계획(이중 모드), 그리고 상세 구현 분석을 다루고 있습니다.

## 1. 주요 이슈 및 개선 제안 (`ISSUES_AND_OPINIONS_KR.md`)

Fablab의 발전을 위해 제기된 주요 이슈와 그에 대한 의견들을 정리한 문서입니다.

*   **K8s 종속성 문제**: `k8s.io/client-go` 등 무거운 라이브러리에 대한 의존성을 줄이고, **Optional Dependency** 또는 **Adapter Pattern**을 적용하여 Core Kernel을 K8s-Agnostic하게 유지해야 함.
*   **배포 방식 (Go Binary)**: 현재 라이브러리 형태에서 나아가, 표준화된 **공용 바이너리(Official Binary)** 제공과 라이브러리 모드를 동시에 지원하는 하이브리드 배포 전략 제안.
*   **상태 저장 (Instance Store)**: 로컬 파일시스템 의존성을 탈피하고, **"State Backend" 추상화**를 통해 K8s Secret, DB 등 다양한 저장소를 지원해야 함.
*   **확장성 (Reconciliation)**: 단순 생성(`Up`)을 넘어 **Diff(Current, Desired)** 알고리즘을 도입하여 스케일링 및 상태 조정을 지원해야 함.
*   **CLI UX**: `kubectl`보다 높은 추상화 레벨의 사용자 친화적 CLI(`create`, `up`, `ssh`)를 유지하고 강화해야 함.

## 2. K8s 중립적 아키텍처 제안 (`K8S_AGNOSTIC_PROPOSAL_KR.md`)

Fablab이 특정 인프라(K8s)에 종속되지 않는 범용 도구로 진화하기 위한 아키텍처 제안입니다.

*   **철학**: "Provider Agnostic Architecture". K8s는 선택지 중 하나여야 함.
*   **Core Kernel**: 인프라 배포 위치를 몰라야 하며, 논리적 모델링과 라이프사이클 관리만 담당.
*   **Runtime Providers**:
    *   **Local Runtime**: 로컬 프로세스/Docker (개발용)
    *   **K8s Runtime**: K8s API Server (운영용)
    *   **SSH Runtime**: 원격 서버 제어 (현재 강점)
*   **Interface 기반 설계**: `ProcessRunner` 등 인터페이스를 정의하고 구현체를 주입.
*   **Build Tags**: `fablab-lite`(No K8s), `fablab-full`(With K8s) 등으로 바이너리 분리 빌드 제안.

## 3. 이중 모드(Dual Mode) 아키텍처 계획 (`plan.plan.md`, `PLAN_ANALYSIS_KR.md`)

Fablab의 사용성을 극대화하기 위해 **코드 기반(Go)**과 **설정 파일 기반(YAML)** 두 가지 방식을 모두 지원하는 계획입니다.

### 3.1 개념
*   **Code-based (기존)**: Go 코드로 모델을 정의. 무한한 자유도, 개발자/파워 유저용.
*   **Config-based (신규)**: YAML로 모델을 정의. 낮은 진입 장벽, GitOps/운영자용.
*   **Hybrid**: 내부적으로는 동일한 **Fablab Kernel**을 공유하여 호환성 유지.

### 3.2 상세 구조
*   **Model Loader**: YAML 파일을 파싱하여 Go `model.Model` 구조체로 변환.
*   **CLI 확장**: `fablab up --config model.yaml`과 같이 `--config` 플래그로 모드 전환.
*   **K8s Operator 통합**: Operator가 YAML 변경을 감지하여 Kernel을 통해 상태 반영.
*   **마이그레이션**: 기존 Go 코드는 그대로 지원하며, 점진적으로 YAML 도입 가능.

### 3.3 평가
*   **장점**: 개발 유연성(Go)과 운영 생산성(YAML)을 모두 확보. 클라우드 네이티브 생태계(GitOps) 통합 용이.
*   **우려**: YAML 문법 복잡도 상승 주의, 파서 유지보수 부담.

## 4. 상세 구현 분석: Controller 설치 (`fablab-controller설치.md`)

Fablab의 Lifecycle을 통해 모델(Controller)이 설치되는 과정을 코드 레벨에서 분석한 문서입니다.

*   **4단계 Lifecycle**:
    1.  **Express (Infrastructure)**: `Expression` 단계. Terraform 등을 통해 VM/인프라 리소스를 생성하고 IP를 바인딩.
    2.  **Build (Configuration)**: 설정 파일 생성, 인증서 발급 등 구성 작업.
    3.  **Sync (Distribution)**: 생성된 파일(바이너리, 설정 등)을 원격 호스트로 전송.
    4.  **Activate (Activation)**: 서비스 시작 및 프로세스 가동.

---
**종합 요약**: Fablab은 현재 Go 언어 기반의 강력한 프로그래밍적 제어 능력을 가지고 있으나, 향후 **K8s 의존성 분리**, **YAML 설정 지원(이중 모드)**, **상태 저장소 추상화** 등을 통해 더 범용적이고 운영 친화적인 "플랫폼"으로 진화하려는 로드맵을 가지고 있습니다.
