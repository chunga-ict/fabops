# Fablab Controller & Router 설치 프로세스 분석 및 적용 방안

본 문서는 `fablab-controller-router설치.md`의 내용을 바탕으로 Fablab의 Controller 및 Router 설치 프로세스를 상세히 분석하고, 이를 새로운 아키텍처(Component Registry, YAML 설정, MCP)에 적용하기 위한 구체적인 방안을 제안합니다.

## 1. 현재 설치 프로세스 분석 (The 4-Stage Lifecycle)

Fablab의 컴포넌트 설치는 `up` 명령어를 통해 4단계 라이프사이클(`Express` → `Build` → `Sync` → `Activate`)로 순차적으로 진행됩니다.

### 1.1 Express (Infrastructure - 인프라 구축)
*   **목적**: VM 인스턴스 생성 및 네트워크 바인딩.
*   **구현**: `kernel/lib/runlevel/0_infrastructure/terraform`. 타겟 클라우드(AWS 등)에 Terraform을 적용하여 인스턴스를 생성하고, Public/Private IP를 추출하여 `Label`에 저장합니다.
*   **결과**: SSH 접속 가능한 빈 VM 상태. `Label.State = Expressed`.

### 1.2 Build (Configuration - 바이너리 준비)
*   **목적**: 로컬 머신에서 배포할 파일(바이너리, 설정, PKI) 준비.
*   **핵심 인터페이스**: `FileStagingComponent.StageFiles(Run, Component)`
*   **동작**:
    1.  **Binaries**: `bin/` 디렉토리에 실행 파일(controller, router) 복사.
    2.  **Config**: `cfg/` 디렉토리에 템플릿 기반 설정 파일(YAML) 생성. IP 주소 등이 이 단계에서 주입됨.
    3.  **Scripts**: `scripts/` 디렉토리에 시작/종료 스크립트 생성.
    4.  **PKI**: `pki/` 디렉토리에 CA 및 인증서 생성.
*   **결과**: 로컬 Staging 영역(`~/.fablab/instances/<id>/kit/build/`)에 모든 파일 준비 완료. `Label.State = Configured`.

### 1.3 Sync (Distribution - 파일 배포)
*   **목적**: 로컬 파일을 원격 VM으로 전송 및 초기 설정.
*   **전송**: `rsync`를 사용하여 Staging 영역을 원격 호스트(`~/fablab/`)로 동기화.
*   **핵심 인터페이스**: `HostInitializingComponent.InitializeHost(Run, Component)`
*   **동작**:
    1.  실행 권한 부여 (`chmod +x`).
    2.  데이터/로그 디렉토리 생성 (`mkdir -p`).
    3.  시스템 커널 파라미터 튜닝 (`sysctl`).
*   **결과**: 원격 호스트에 실행 가능한 상태로 파일 배치 완료. `Label.State = Distributed`.

### 1.4 Activate (Activation - 프로세스 시작)
*   **목적**: 실제 프로세스 구동 및 서비스 개시.
*   **핵심 인터페이스**: `ServerComponent.Start(Run, Component)`
*   **동작**:
    1.  기존 프로세스 종료 (Kill).
    2.  `nohup` 또는 `systemd`를 사용하여 백그라운드 프로세스 시작.
    3.  `IsRunning` 체크를 통한 실행 확인.
*   **결과**: 서비스 정상 가동. `Label.State = Activated`.

---

## 2. 신규 아키텍처 적용 방안 (Registry & YAML)

Global 변수 제거 및 Component Registry 도입에 따라, Controller와 Router는 동적으로 로드 가능한 **플러그인 컴포넌트**로 재구현되어야 합니다.

### 2.1 Component Registry 등록

외부 프로젝트(예: `ziti-fablab-plugins`)에서 각 컴포넌트 구조체를 정의하고 커널 Registry에 등록합니다.

```go
// implementation/ziti/controller.go

type ZitiController struct {
    Version string
}

func init() {
    // Registry에 "ziti-controller" 타입 등록
    model.RegisterComponentType("ziti-controller", func() model.ComponentType {
        return &ZitiController{Version: "latest"}
    })
}

// 1. Build Phase: 파일 준비
func (c *ZitiController) StageFiles(run model.Run, cmp *model.Component) error {
    // 로컬 bin/cfg/pki 생성 로직
    return buildControllerKit(run, cmp) 
}

// 2. Sync Phase: 호스트 초기화
func (c *ZitiController) InitializeHost(run model.Run, cmp *model.Component) error {
    host := cmp.GetHost()
    return host.ExecLogged("chmod +x ~/fablab/bin/ziti-controller")
}

// 3. Activate Phase: 서비스 시작
func (c *ZitiController) Start(run model.Run, cmp *model.Component) error {
    host := cmp.GetHost()
    return host.ExecLogged("nohup ~/fablab/bin/ziti-controller run ~/fablab/cfg/controller.yml &")
}

// 실행 상태 확인
func (c *ZitiController) IsRunning(run model.Run, cmp *model.Component) (bool, error) {
    return libssh.IsProcessRunning(cmp.GetHost(), "ziti-controller")
}
```

### 2.2 YAML 설정을 통한 정의 (선언적 배포)

사용자는 YAML 파일에 컴포넌트 타입을 명시하여 배포를 정의합니다.

```yaml
# fablab.yaml
regions:
  us-east-1:
    hosts:
      host-ctrl:
        components:
          - type: ziti-controller  # Registry Lookup
            config:
              listen_address: 0.0.0.0
      host-router:
        components:
          - type: ziti-router      # Registry Lookup
            config:
              mode: edge
              ctrl_endpoint: host-ctrl
```

### 2.3 제네릭 컴포넌트 활용 (과도기적 방안)

전용 구현체가 없는 경우, 쉘 스크립트를 주입하여 동작시키는 `GenericComponent`를 활용합니다. 이를 통해 기존 쉘 스크립트 자산을 재활용할 수 있습니다.

```yaml
components:
  - type: generic
    config:
      build_script: "scripts/build_router.sh"
      stage_files: 
        - src: "bin/ziti-router"
          dst: "bin/ziti-router"
      start_command: "./bin/ziti-router run config.yml"
```

## 3. 요약 및 제언

1.  **인터페이스 준수**: 새로운 컴포넌트 구현 시 `FileStagingComponent`, `HostInitializingComponent`, `ServerComponent` 인터페이스를 충실히 구현해야 4단계 라이프사이클에 완벽하게 통합됩니다.
2.  **구조 분리**: `fablab/kernel`은 순수한 실행 엔진(Orchestrator)으로 남고, 실제 `Ziti` 관련 비즈니스 로직(PKI 생성, Config 템플릿 등)은 별도 패키지로 분리하여 Registry를 통해 주입하는 구조가 이상적입니다.
3.  **자동화 확장**: MCP 서버는 `create_network` 요청 시 이 YAML 구성을 생성하고 `up` 명령을 트리거하여, 대화형 인터페이스만으로 복잡한 인프라와 서비스를 배포할 수 있게 됩니다.
