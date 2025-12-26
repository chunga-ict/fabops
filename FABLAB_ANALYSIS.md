# Fablab 구조 및 코드 분석

## 1. 프로젝트 개요
**Fablab**은 대규모 분산 배포 모델을 위한 운영 개발 도구 세트입니다. 주로 [Ziti](https://ziti.dev) 네트워크와 애플리케이션의 개발, 배포, 연구 및 관리를 위해 설계되었습니다. 단순한 DSL(Yaml, JSON) 설정 파일 대신 **Go 언어 자체를 프로그래밍 도구**로 사용하여 복잡한 운영 시나리오를 코드로 표현하고 실행할 수 있게 합니다.

## 2. 디렉토리 구조
프로젝트의 주요 디렉토리 구조는 다음과 같습니다.

*   `cmd/`: CLI 애플리케이션의 진입점(Entrypoint)입니다.
    *   `fablab/`: 메인 `fablab` 명령어 소스 코드가 위치합니다.
        *   `subcmd/`: `up`, `down`, `ssh`, `create` 등 개별 하위 명령어(subcommand)의 구현체가 있습니다.
*   `kernel/`: Fablab의 핵심 로직과 라이브러리가 포함된 "커널"입니다.
    *   `model/`: 프로젝트의 가장 중요한 부분으로, 시스템을 정의하는 데이터 구조와 인터페이스(`Model`, `Region`, `Host`, `Component` 등)가 정의되어 있습니다.
    *   `lib/`: 다양한 유틸리티 라이브러리 모음입니다.
    *   `libssh/`: SSH 연결 및 원격 제어를 위한 라이브러리입니다.
*   `docs/`: 프로젝트 문서 및 이미지가 포함되어 있습니다.
*   `resources/`: 리소스 파일들이 위치합니다.

## 3. 핵심 아키텍처 (The Model)

Fablab은 모든 환경을 "모델(`model.Model`)"로 표현하며, 이는 크게 세 가지 개념적 레이어로 나뉩니다.

### A. 구조적 모델 (The Structural Model)
분산 환경의 "디지털 트윈"을 유지합니다. `golang` 데이터 구조로 표현되며, 배포하려는 환경의 구조와 설정을 정의합니다.
*   **주요 엔티티**: `Model` (루트), `Region` (지역), `Host` (호스트/VM), `Component` (소프트웨어 컴포넌트)
*   계층적 구조: Model -> Region -> Host -> Component

### B. 행위적 모델 (The Behavioral Model)
구조적 모델에 대해 수행되는 로직과 동작을 정의합니다.
*   **Factories**: 모델 자체를 구축하고 구성하는 역할을 합니다.
*   **Lifecycle Stages (생명주기 단계)**: 모델 인스턴스가 거치는 단계들입니다.
    *   `Infrastructure`: 인프라 프로비저닝
    *   `Configuration`: 설정
    *   `Distribution`: 배포
    *   `Activation`: 활성화
    *   `Operation`: 운영
    *   `Disposal`: 폐기
*   **Actions**: 특정 생명주기와 무관하게 모델에 대해 수행할 수 있는 개별 작업들입니다.

### C. 인스턴스 모델 (The Instance Model)
모델의 구체적인 실행 인스턴스를 관리합니다. 여러 호스팅 환경에서 모델을 인스턴스화하고 운영 데이터와 상태를 관리합니다.

## 4. 주요 모듈 및 코드 분석

### `kernel/model/model.go` (핵심 데이터 구조)
이 파일은 Fablab의 뼈대가 되는 구조체를 정의합니다.
*   **`Model` struct**: 전체 시스템의 진입점입니다. `Scope`, `Regions`, `Lifecycle Stages`(Infrastructure, Configuration 등)을 포함합니다.
*   **`Region`, `Host` struct**: 인프라의 물리적/논리적 단위를 나타냅니다. `Host` 구조체는 SSH 클라이언트, EC2 정보, 공인/사설 IP 등을 관리하며 원격 명령 실행(`ExecLogged`) 기능을 제공합니다.
*   **`VarConfig`**: 환경 변수, 커맨드라인 인자, 바인딩 등을 통합 관리하는 변수 설정 구조체입니다.
*   **`Entity` 인터페이스**: 모든 모델 요소가 구현하는 인터페이스로, 계층 탐색(부모/자식 찾기) 및 변수 조회 기능을 표준화합니다.

### `cmd/fablab/subcmd/` (CLI 명령어)
Cobara 라이브러리를 사용하여 CLI 명령어들을 구현합니다.
*   `up.go`, `down.go`: 전체 라이프사이클을 실행하거나 종료합니다.
*   `ssh.go`: 특정 호스트에 SSH로 접속합니다.
*   `create.go`, `build.go`: 모델 인스턴스를 생성하고 빌드합니다.

## 5. 기술 스택
*   **언어**: Go (Golang)
*   **CLI 프레임워크**: `spf13/cobra`
*   **원격 제어**: `golang.org/x/crypto/ssh` (SSH 클라이언트 구현)
*   **클라우드 연동**: `aws-sdk-go` (AWS 제어)
*   **기타**: `influxdb-client-go` (메트릭 저장), `logrus` (로깅)

## 6. 요약
Fablab은 Go 언어의 강력함을 활용하여 복잡한 분산 네트워크(Ziti 등)를 코드로 정의(IaC)하고, 배포부터 테스트, 운영까지 전 생명주기를 관리하는 "프로그래밍 가능한 운영 프레임워크"입니다. 단순 스크립트의 한계를 넘어, 구조화된 모델링과 계층화된 아키텍처를 통해 확장성과 유지보수성을 확보하고 있습니다.
