# Fablab 분석
##  **Model.Model에서 Controller (컴포넌트) 설치 프로세스 - 세부 분석**

fablab에서 controller를 포함한 모든 컴포넌트는 **4단계 Lifecycle**을 거쳐 설치됩니다. `fablab up` 명령 실행 시 전체 프로세스가 진행됩니다.

---

##  **전체 설치 Lifecycle 개요**

```32:69:cmd/fablab/subcmd/up.go
func up(_ *cobra.Command, _ []string) {
	if err := model.Bootstrap(); err != nil {
		logrus.Fatalf("unable to bootstrap (%v)", err)
	}

	ctx, err := model.NewRun()
	if err != nil {
		logrus.WithError(err).Fatal("error initializing run")
	}

	figlet.Figlet("infrastructure")

	if err := ctx.GetModel().Express(ctx); err != nil {
		logrus.Fatalf("error expressing (%v)", err)
	}

	figlet.Figlet("configuration")

	if err := ctx.GetModel().Build(ctx); err != nil {
		logrus.Fatalf("error building (%v)", err)
	}

	figlet.Figlet("distribution")

	if err := ctx.GetModel().Sync(ctx); err != nil {
		logrus.Fatalf("error distributing (%v)", err)
	}

	figlet.Figlet("activation")

	if err := ctx.GetModel().Activate(ctx); err != nil {
		logrus.Fatalf("error activating (%v)", err)
	}

	figlet.Figlet("FABUL0US!1!")
}
```

**실행 순서**: `Express` → `Build` → `Sync` → `Activate`

---

##  **단계 1: Express (Infrastructure - 인프라 구축)**

### 목적
원격 클라우드에 VM 인스턴스를 생성하고 IP 주소를 바인딩합니다.

### 세부 프로세스

```1125:1136:kernel/model/model.go
func (m *Model) Express(run Run) error {
	for _, stage := range m.Infrastructure {
		if err := stage.Execute(run); err != nil {
			return fmt.Errorf("error expressing infrastructure (%w)", err)
		}
	}
	run.GetLabel().State = Expressed
	if err := run.GetLabel().Save(); err != nil {
		return fmt.Errorf("error updating instance label (%w)", err)
	}
	return nil
}
```

#### 1.1 Terraform을 사용한 인프라 프로비저닝

```49:87:kernel/lib/runlevel/0_infrastructure/terraform/terraform.go
func (t *Terraform) Execute(run model.Run) error {
	m := run.GetModel()
	l := run.GetLabel()

	if err := t.generate(m); err != nil {
		return err
	}

	attemptsRemaining := t.Retries + 1

	var err error
	for attemptsRemaining > 0 {
		err = t.Init()

		if err == nil {
			err = t.apply()
		}

		if err == nil {
			err = t.bind(m, l)
		}

		if err == nil && t.ReadyCheck != nil {
			err = t.ReadyCheck.Execute(run)
		}

		if err == nil {
			return nil
		}

		attemptsRemaining--
		if attemptsRemaining > 0 {
			pfxlog.Logger().WithError(err).Error("terraform failure, retrying in 3s")
			time.Sleep(3 * time.Second)
		}
	}

	return err
}
```

**세부 작업**:
1. **`generate(m)`**: Model 정의를 바탕으로 Terraform `.tf` 파일 생성
2. **`terraform init`**: Terraform 초기화 (프로바이더 다운로드)
3. **`terraform apply`**: AWS/GCP 등에 VM 인스턴스 실제 생성
4. **`bind(m, l)`**: Terraform output에서 Public/Private IP를 추출하여 Model과 Label에 바인딩
5. **`ReadyCheck`**: SSH 접속 가능 여부 확인 (최대 90초 대기)
6. **재시도**: 실패 시 최대 3회 재시도

#### 1.2 Label 상태 업데이트

```yaml
# ~/.fablab/instances/<instance-id>/fablab.yml
id: my-instance
model: my-model
state: Expressed  # ← 상태 업데이트
bindings:
  region1_host_ctrl01_public_ip: "54.123.45.67"
  region1_host_ctrl01_private_ip: "10.0.1.10"
```

**결과**: 
-  클라우드에 VM 인스턴스 생성 완료
-  IP 주소 매핑 완료
-  SSH 접속 가능 상태

---

##  **단계 2: Build (Configuration - 바이너리 준비)**

### 목적
Controller 바이너리, 설정 파일, 스크립트 등을 로컬에서 준비합니다.

### 세부 프로세스

```1138:1160:kernel/model/model.go
func (m *Model) Build(run Run) error {
	err := m.ForEachComponent("*", 1, func(c *Component) error {
		if stageable, ok := c.Type.(FileStagingComponent); ok {
			return stageable.StageFiles(run, c)
		}
		return nil
	})

	if err != nil {
		return err
	}

	for _, stage := range m.Configuration {
		if err := stage.Execute(run); err != nil {
			return fmt.Errorf("error building configuration (%w)", err)
		}
	}
	run.GetLabel().State = Configured
	if err := run.GetLabel().Save(); err != nil {
		return fmt.Errorf("error updating instance label (%w)", err)
	}
	return nil
}
```

#### 2.1 각 컴포넌트의 StageFiles 호출

```67:73:kernel/model/component.go
type FileStagingComponent interface {
	ComponentType

	// StageFiles is called at the beginning of the configuration phase and allows the component to contribute
	// files to be synced to the Host
	StageFiles(r Run, c *Component) error
}
```

**Controller의 StageFiles 구현 예시** (실제 구현체는 별도 프로젝트):
```go
func (ctrl *ControllerType) StageFiles(r Run, c *Component) error {
    // 1. 바이너리 복사
    //    ~/.fablab/instances/<instance>/kit/build/bin/controller
    
    // 2. 설정 파일 생성
    //    ~/.fablab/instances/<instance>/kit/build/cfg/controller.yml
    
    // 3. 시작 스크립트 생성
    //    ~/.fablab/instances/<instance>/kit/build/scripts/start-controller.sh
    
    // 4. PKI 인증서 복사 (필요 시)
    //    ~/.fablab/instances/<instance>/kit/build/pki/...
}
```

#### 2.2 로컬 Staging 디렉토리 구조

```
~/.fablab/instances/<instance-id>/kit/build/
├── bin/
│   ├── controller          # Controller 바이너리
│   ├── router              # Router 바이너리 (다른 컴포넌트 예시)
│   └── ziti                # CLI 도구
├── cfg/
│   ├── controller.yml      # Controller 설정 파일
│   └── router.yml          # Router 설정 파일
├── scripts/
│   ├── start-controller.sh # 시작 스크립트
│   └── stop-controller.sh  # 종료 스크립트
└── pki/
    ├── ca/                 # CA 인증서
    ├── server/             # 서버 인증서
    └── client/             # 클라이언트 인증서
```

**결과**:
-  모든 파일이 로컬 staging 디렉토리에 준비됨
-  설정 파일이 템플릿으로 생성됨 (IP 주소, 포트 등 포함)
-  Label 상태: `Configured`

---

##  **단계 3: Sync (Distribution - 파일 배포)**

### 목적
로컬에 준비된 파일들을 원격 VM으로 전송합니다.

### 세부 프로세스

```1162:1193:kernel/model/model.go
func (m *Model) Sync(run Run) error {
	for idx, stage := range m.Distribution {
		if err := stage.Execute(run); err != nil {
			return fmt.Errorf("error distributing stage %d - %T, (%w)", idx+1, stage, err)
		}
	}

	err := m.ForEachHost("*", 100, func(host *Host) error {
		for _, c := range host.Components {
			hostInitializer, ok := c.Type.(HostInitializingComponent)
			if !ok {
				continue
			}

			if err := hostInitializer.InitializeHost(run, c); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	run.GetLabel().State = Distributed
	if err := run.GetLabel().Save(); err != nil {
		return fmt.Errorf("error updating instance label (%w)", err)
	}

	return nil
}
```

#### 3.1 rsync를 통한 파일 전송

```172:198:kernel/lib/runlevel/3_distribution/rsync/rsync.go
func (self *localRsyncer) run() error {
	for {
		host, left, current := self.GetNextHostPreferringNotRegions(self.regions)
		if host == nil {
			return nil
		}
		logrus.Infof("syncing local -> %v. Left: %v, current: %v", host.PublicIp, left, current)
		config := NewConfig(host)
		if err := synchronizeHost(self.rsyncContext, config); err != nil {
			return errors.Wrapf(err, "error synchronizing host [%s/%s]", host.GetRegion().GetId(), host.GetId())
		}
		left, current = self.markDone()
		logrus.Infof("finished syncing local -> %v. Left: %v, current: %v", host.PublicIp, left, current)

		if err := self.ctx.Err(); err != nil {
			logrus.WithError(err).Info("exiting sync early as group context is failed")
			return err
		}

		remoteSyncer := &remoteRsyncer{
			host:         host,
			rsyncContext: self.rsyncContext,
		}

		self.group.Go(remoteSyncer.run)
	}
}
```

**전송 전략**:
1. **Local → Host1**: 로컬에서 첫 번째 호스트로 rsync
2. **Host1 → Host2**: 첫 번째 호스트에서 같은 리전의 다른 호스트로 병렬 전송
3. **Host1 → Host3 (다른 리전)**: 다른 리전으로도 병렬 전송
4. **최적화**: 네트워크 대역폭 절약을 위해 tree 구조로 전파

**rsync 명령어 예시**:
```bash
rsync -avz --delete \
  -e "ssh -i ~/.ssh/fablab_rsa -o StrictHostKeyChecking=no" \
  ~/.fablab/instances/my-instance/kit/build/ \
  ubuntu@54.123.45.67:/home/ubuntu/fablab/
```

#### 3.2 InitializeHost 호출

```78:84:kernel/model/component.go
type HostInitializingComponent interface {
	ComponentType

	// InitializeHost is called at the end of the distribution phase and allows the component to
	// make changes to Host configuration
	InitializeHost(r Run, c *Component) error
}
```

**Controller의 InitializeHost 구현 예시**:
```go
func (ctrl *ControllerType) InitializeHost(r Run, c *Component) error {
    host := c.GetHost()
    
    // 1. 실행 권한 설정
    host.ExecLogged("chmod +x /home/ubuntu/fablab/bin/controller")
    host.ExecLogged("chmod +x /home/ubuntu/fablab/scripts/*.sh")
    
    // 2. 시스템 설정 변경 (sysctl, ulimit 등)
    host.ExecLogged("sudo sysctl -w net.core.somaxconn=4096")
    
    // 3. 디렉토리 권한 설정
    host.ExecLogged("mkdir -p /home/ubuntu/fablab/data")
    host.ExecLogged("mkdir -p /home/ubuntu/fablab/logs")
    
    return nil
}
```

**원격 호스트 파일 구조**:
```
/home/ubuntu/fablab/
├── bin/
│   ├── controller*         # 실행 권한 부여됨
│   ├── router*
│   └── ziti*
├── cfg/
│   ├── controller.yml
│   └── router.yml
├── scripts/
│   ├── start-controller.sh*
│   └── stop-controller.sh*
├── pki/
│   └── ...
├── data/                   # InitializeHost에서 생성
└── logs/                   # InitializeHost에서 생성
```

**결과**:
-  모든 파일이 원격 VM에 전송됨
-  실행 권한 및 디렉토리 구조 설정 완료
-  Label 상태: `Distributed`

---

##  **단계 4: Activate (Activation - 프로세스 시작)**

### 목적
Controller 프로세스를 실제로 시작하여 실행 상태로 만듭니다.

### 세부 프로세스

```1195:1206:kernel/model/model.go
func (m *Model) Activate(run Run) error {
	for _, stage := range m.Activation {
		if err := stage.Execute(run); err != nil {
			return fmt.Errorf("error activating (%w)", err)
		}
	}
	run.GetLabel().State = Activated
	if err := run.GetLabel().Save(); err != nil {
		return fmt.Errorf("error updating instance label (%w)", err)
	}
	return nil
}
```

#### 4.1 Component Start 호출

Model의 Activation Stages에 등록된 액션들이 실행됩니다. 일반적으로:

```go
// Model 정의 시 (별도 프로젝트)
m.AddActivationAction("start-controllers")  // controller 컴포넌트 시작
m.AddActivationAction("start-routers")      // router 컴포넌트 시작
m.AddActivationAction("init-network")       // 네트워크 초기화
```

#### 4.2 ServerComponent.Start 구현

```59:62:kernel/model/component.go
// A ServerComponent is one which can be started and left running in the background
type ServerComponent interface {
	Start(run Run, c *Component) error
}
```

**Controller의 Start 구현 예시**:
```go
func (ctrl *ControllerType) Start(r Run, c *Component) error {
    host := c.GetHost()
    
    // 1. 기존 프로세스 종료 (있다면)
    host.KillProcesses("-9", func(line string) bool {
        return strings.Contains(line, "controller")
    })
    
    // 2. nohup으로 백그라운드 실행
    startCmd := fmt.Sprintf(
        "nohup /home/ubuntu/fablab/bin/controller run "+
        "--log-formatter pfxlog "+
        "/home/ubuntu/fablab/cfg/controller.yml "+
        "> /home/ubuntu/fablab/logs/controller.log 2>&1 &",
    )
    
    output, err := host.ExecLogged(startCmd)
    if err != nil {
        return fmt.Errorf("failed to start controller: %w", err)
    }
    
    // 3. 프로세스 시작 확인 (몇 초 대기)
    time.Sleep(3 * time.Second)
    
    // 4. Health Check
    running, err := c.IsRunning(r)
    if !running {
        return fmt.Errorf("controller failed to start")
    }
    
    logrus.Infof("Controller started successfully on %s", host.PublicIp)
    return nil
}
```

#### 4.3 실행 확인

```254:259:kernel/model/component.go
func (component *Component) IsRunning(run Run) (bool, error) {
	if component.Type == nil {
		return false, errors.Errorf("component [%s] has no component type defined", component.Id)
	}
	return component.Type.IsRunning(run, component)
}
```

**IsRunning 구현 예시**:
```go
func (ctrl *ControllerType) IsRunning(r Run, c *Component) (bool, error) {
    host := c.GetHost()
    
    // ps ax로 프로세스 확인
    output, err := host.ExecLogged("ps ax | grep controller | grep -v grep")
    if err != nil {
        return false, nil
    }
    
    return len(output) > 0, nil
}
```

**결과**:
-  Controller 프로세스가 백그라운드에서 실행 중
-  로그 파일 생성: `/home/ubuntu/fablab/logs/controller.log`
-  Label 상태: `Activated`

---

##  **추가: 컴포넌트 재시작 및 관리**

### 컴포넌트 중지

```bash
fablab stop ctrl01
```

```41:48:kernel/lib/actions/component/stop.go
func (stop *stop) Execute(run model.Run) error {
	return run.GetModel().ForEachComponent(stop.componentSpec, stop.concurrency, func(c *model.Component) error {
		if c.Type != nil {
			return c.Type.Stop(run, c)
		}
		return nil
	})
}
```

### 컴포넌트 시작

```bash
fablab start ctrl01
```

```34:41:kernel/lib/actions/component/start.go
func (start *start) Execute(run model.Run) error {
	return run.GetModel().ForEachComponent(start.componentSpec, start.concurrency, func(c *model.Component) error {
		if startable, ok := c.Type.(ServerComponent); ok {
			return startable.Start(run, c)
		}
		return nil
	})
}
```

### 컴포넌트 재시작

```bash
fablab restart ctrl01
```

```49:69:cmd/fablab/subcmd/restart.go
func (self *restartAction) run(_ *cobra.Command, args []string) {
	if err := model.Bootstrap(); err != nil {
		logrus.Fatalf("unable to bootstrap (%s)", err)
	}

	ctx, err := model.NewRun()
	if err != nil {
		logrus.WithError(err).Fatal("error initializing run")
	}

	if err = component.StopInParallel(args[0], self.concurrency).Execute(ctx); err != nil {
		logrus.WithError(err).Fatalf("error stopping components")
	}

	if err = component.StartInParallel(args[0], self.concurrency).Execute(ctx); err != nil {
		logrus.WithError(err).Fatalf("error starting components")
	}

	c := ctx.GetModel().SelectComponents(args[0])
	logrus.Infof("%d components restarted", len(c))
}
```

---

##  **전체 프로세스 요약 다이어그램**

```
┌─────────────────────────────────────────────────────────────┐
│ fablab up                                                     │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────────────────────────┐
│ 1. Express (Infrastructure)                                │
├────────────────────────────────────────────────────────────┤
│  ? Terraform generate (.tf 파일 생성)                        │
│  ? terraform init                                          │
│  ? terraform apply (AWS/GCP VM 생성)                        │
│  ? bind (IP 주소 매핑)                                       │
│  ? ReadyCheck (SSH 접속 확인)                                │
│  → Label.State = Expressed                                 │
└────────────────┬───────────────────────────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────────────────────────┐
│ 2. Build (Configuration)                                   │
├────────────────────────────────────────────────────────────┤
│  ? ForEachComponent: StageFiles 호출                        │
│    - Controller 바이너리 복사                                │
│    - 설정 파일 생성 (controller.yml)                         │
│    - 시작 스크립트 생성                                       │
│    - PKI 인증서 복사                                         │
│  → 로컬 staging 디렉토리 준비 완료                            │
│  → Label.State = Configured                                │
└────────────────┬───────────────────────────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────────────────────────┐
│ 3. Sync (Distribution)                                     │
├────────────────────────────────────────────────────────────┤
│  ? rsync: 로컬 → VM 파일 전송                                │
│  ? ForEachHost: InitializeHost 호출                        │
│    - chmod +x (실행 권한 부여)                               │
│    - mkdir (디렉토리 생성)                                   │
│    - sysctl (시스템 설정)                                    │
│  → 원격 VM에 모든 파일 배포 완료                              │
│  → Label.State = Distributed                               │
└────────────────┬───────────────────────────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────────────────────────┐
│ 4. Activate (Activation)                                   │
├────────────────────────────────────────────────────────────┤
│  ? Activation Stages 실행                                   │
│  ? ServerComponent.Start 호출                              │
│    - 기존 프로세스 종료                                       │
│    - nohup으로 백그라운드 실행                                │
│    - Health Check                                          │
│  → Controller 프로세스 실행 중                                │
│  → Label.State = Activated                                 │
└────────────────────────────────────────────────────────────┘
```

---

##  **핵심 인터페이스 정리**

| 인터페이스 | 단계 | 역할 |
|-----------|------|------|
| **ComponentType** | 공통 | 모든 컴포넌트가 구현해야 하는 기본 인터페이스 |
| **FileStagingComponent** | Build | 바이너리, 설정 파일 준비 (`StageFiles`) |
| **HostInitializingComponent** | Sync | 원격 호스트 초기 설정 (`InitializeHost`) |
| **ServerComponent** | Activate | 프로세스 시작 (`Start`) |
| **InitializingComponent** | Activate | 네트워크 초기화 등 (`Init`) |

---

이제 controller (또는 모든 컴포넌트)가 어떻게 설치되는지 완전히 이해하셨을 것입니다!


##  **"실제 구현체는 별도 프로젝트"의 의미**

fablab은 **프레임워크(Framework)**이고, 실제 Controller 같은 컴포넌트의 구체적인 구현은 **fablab을 사용하는 별도의 Go 프로젝트**에서 이루어집니다.

---

##  **프로젝트 구조 관계**

### 1 **fablab 프로젝트 (현재 저장소)**

**역할**: 프레임워크/라이브러리 제공

```
github.com/openziti/fablab/
├── kernel/
│   ├── model/
│   │   ├── model.go          # Model, Region, Host 정의
│   │   ├── component.go      # ComponentType 인터페이스 정의
│   │   └── ...
│   ├── lib/
│   │   ├── runlevel/         # Express, Build, Sync, Activate 구현
│   │   └── ...
│   └── libssh/               # SSH 통신 유틸리티
├── cmd/fablab/
│   └── main.go               # fablab CLI 진입점
└── main.go                   # 외부에서 import할 진입 함수
```

```9:17:main.go
func InitModel(m *model.Model) {
	model.InitModel(m)
}

func Run() {
	if err := subcmd.RootCmd.Execute(); err != nil {
		logrus.WithError(err).Fatal("failure")
	}
}
```

**제공하는 것**:
-  `model.ComponentType` 인터페이스
-  `model.FileStagingComponent` 인터페이스
-  `model.ServerComponent` 인터페이스
-  Lifecycle 관리 (`Express`, `Build`, `Sync`, `Activate`)
-  SSH, rsync, Terraform 통합
-  CLI 프레임워크

**제공하지 않는 것**:
-  Controller의 구체적인 구현
-  Router의 구체적인 구현
-  특정 애플리케이션의 비즈니스 로직

---

### 2 **실제 구현 프로젝트 (예: ziti-test)**

**역할**: fablab을 **import**하여 실제 Controller 구현

```
github.com/openziti/ziti-test/
├── go.mod
│   require github.com/openziti/fablab v0.x.x
│
├── models/
│   └── simple/
│       └── main.go           # Model 정의 및 진입점
│
├── zitilib/
│   ├── controller.go         # Controller ComponentType 구현 
│   ├── router.go             # Router ComponentType 구현
│   └── runlevel/
│       ├── build.go          # Build 단계 커스터마이징
│       └── activate.go       # Activate 단계 커스터마이징
│
└── build/                    # 빌드된 fablab 바이너리
    └── fablab-simple         # 이 프로젝트 전용 fablab 실행 파일
```

---

##  **실제 Controller 구현 예시**

### **zitilib/controller.go** (별도 프로젝트)

```go
package zitilib

import (
    "fmt"
    "path/filepath"
    "github.com/openziti/fablab/kernel/model"
    "github.com/openziti/fablab/kernel/libssh"
)

// ControllerType은 Ziti Controller 컴포넌트의 구체적인 구현
type ControllerType struct {
    Version      string
    BinaryPath   string  // 로컬에서 controller 바이너리 경로
    ConfigSource string  // 설정 템플릿 경로
}

// ComponentType 인터페이스 구현 (fablab이 정의)
func (c *ControllerType) Label() string {
    return "ziti-controller"
}

func (c *ControllerType) GetVersion() string {
    return c.Version
}

func (c *ControllerType) Dump() any {
    return map[string]string{
        "version": c.Version,
        "binary":  c.BinaryPath,
    }
}

// FileStagingComponent 인터페이스 구현 (fablab이 정의)
func (c *ControllerType) StageFiles(run model.Run, comp *model.Component) error {
    // 1. Controller 바이너리 복사
    binDest := filepath.Join(model.KitBuild(), "bin", "ziti-controller")
    if err := copyFile(c.BinaryPath, binDest); err != nil {
        return fmt.Errorf("failed to copy binary: %w", err)
    }
    
    // 2. 설정 파일 생성 (템플릿 + 변수 치환)
    configDest := filepath.Join(model.KitBuild(), "cfg", "controller.yml")
    config := generateConfig(comp, run.GetLabel())
    if err := writeFile(configDest, config); err != nil {
        return fmt.Errorf("failed to write config: %w", err)
    }
    
    // 3. 시작 스크립트 생성
    scriptDest := filepath.Join(model.KitBuild(), "scripts", "start-controller.sh")
    script := `#!/bin/bash
/home/ubuntu/fablab/bin/ziti-controller run /home/ubuntu/fablab/cfg/controller.yml
`
    if err := writeFile(scriptDest, []byte(script)); err != nil {
        return fmt.Errorf("failed to write script: %w", err)
    }
    
    // 4. PKI 인증서 생성 및 복사
    pkiDest := filepath.Join(model.KitBuild(), "pki")
    if err := generatePKI(comp, pkiDest); err != nil {
        return fmt.Errorf("failed to generate PKI: %w", err)
    }
    
    return nil
}

// HostInitializingComponent 인터페이스 구현 (fablab이 정의)
func (c *ControllerType) InitializeHost(run model.Run, comp *model.Component) error {
    host := comp.GetHost()
    
    // 실행 권한 부여
    cmds := []string{
        "chmod +x /home/ubuntu/fablab/bin/ziti-controller",
        "chmod +x /home/ubuntu/fablab/scripts/*.sh",
        "mkdir -p /home/ubuntu/fablab/data/ctrl",
        "mkdir -p /home/ubuntu/fablab/logs",
    }
    
    for _, cmd := range cmds {
        if _, err := host.ExecLogged(cmd); err != nil {
            return fmt.Errorf("failed to exec %s: %w", cmd, err)
        }
    }
    
    return nil
}

// ServerComponent 인터페이스 구현 (fablab이 정의)
func (c *ControllerType) Start(run model.Run, comp *model.Component) error {
    host := comp.GetHost()
    
    // 1. 기존 프로세스 종료
    host.KillProcesses("-9", func(line string) bool {
        return strings.Contains(line, "ziti-controller")
    })
    
    // 2. nohup으로 백그라운드 실행
    startCmd := "nohup /home/ubuntu/fablab/scripts/start-controller.sh " +
                "> /home/ubuntu/fablab/logs/controller.log 2>&1 &"
    
    if _, err := host.ExecLogged(startCmd); err != nil {
        return fmt.Errorf("failed to start controller: %w", err)
    }
    
    // 3. 시작 확인
    time.Sleep(3 * time.Second)
    
    // 4. Health Check
    running, err := c.IsRunning(run, comp)
    if err != nil || !running {
        return fmt.Errorf("controller failed to start")
    }
    
    logrus.Infof("? Controller started on %s", host.PublicIp)
    return nil
}

func (c *ControllerType) IsRunning(run model.Run, comp *model.Component) (bool, error) {
    host := comp.GetHost()
    output, _ := host.ExecLogged("ps ax | grep ziti-controller | grep -v grep")
    return len(output) > 0, nil
}

func (c *ControllerType) Stop(run model.Run, comp *model.Component) error {
    host := comp.GetHost()
    return host.KillProcesses("-15", func(line string) bool {
        return strings.Contains(line, "ziti-controller")
    })
}

// 헬퍼 함수들
func generateConfig(comp *model.Component, label *model.Label) []byte {
    // 실제 Ziti Controller 설정 파일 생성 로직
    // IP 주소, 포트, 인증서 경로 등 바인딩
    return []byte("...")
}

func generatePKI(comp *model.Component, dest string) error {
    // PKI 인증서 생성 로직
    return nil
}
```

---

### **models/simple/main.go** (별도 프로젝트)

```go
package main

import (
    "github.com/openziti/fablab"           // fablab 프레임워크 import
    "github.com/openziti/fablab/kernel/model"
    "github.com/openziti/ziti-test/zitilib" // 위에서 구현한 Controller
)

func main() {
    // Model 생성
    m := &model.Model{
        Id: "simple",
        Regions: model.Regions{
            "us-east-1": {
                Hosts: model.Hosts{
                    "ctrl01": {
                        InstanceType: "t3.medium",
                        Components: model.Components{
                            "controller": {
                                Type: &zitilib.ControllerType{  // ← 실제 구현체 사용
                                    Version:    "v0.30.0",
                                    BinaryPath: "/path/to/ziti-controller",
                                },
                            },
                        },
                    },
                },
            },
        },
    }
    
    // fablab 프레임워크 초기화
    fablab.InitModel(m)
    
    // fablab CLI 실행
    fablab.Run()
}
```

---

##  **빌드 및 실행 과정**

### 1. 별도 프로젝트에서 fablab 바이너리 빌드

```bash
cd github.com/openziti/ziti-test/models/simple

# main.go를 빌드하면 fablab 프레임워크가 포함된 실행 파일 생성
go build -o ../../build/fablab-simple main.go
```

**생성된 바이너리**: `fablab-simple`
- fablab 프레임워크 코드 포함
- ControllerType 구현 코드 포함 
- Model 정의 포함 

### 2. fablab 인스턴스 생성

```bash
./build/fablab-simple create -n my-ziti-network
```

이 명령이 실행되면:
```yaml
# ~/.fablab/config.yml
instances:
  my-ziti-network:
    name: my-ziti-network
    model: simple
    working_directory: /home/user/.fablab/instances/my-ziti-network
    executable: /path/to/ziti-test/build/fablab-simple  # ← 빌드한 바이너리
default: my-ziti-network
```

### 3. 배포 실행

```bash
# fablab CLI (메인 바이너리)는 자식 프로세스로 fablab-simple 실행
fablab up

# 실제로는 이렇게 실행됨:
# /path/to/ziti-test/build/fablab-simple up
```

**실행 흐름**:
1. `fablab up` 실행
2. `config.yml`에서 `executable` 경로 확인
3. `/path/to/ziti-test/build/fablab-simple up` 자식 프로세스로 실행
4. fablab 프레임워크가 Lifecycle 실행:
   - **Express**: Terraform으로 VM 생성
   - **Build**: `ControllerType.StageFiles()` 호출 ← **여기서 실제 구현 실행**
   - **Sync**: rsync로 파일 전송, `ControllerType.InitializeHost()` 호출
   - **Activate**: `ControllerType.Start()` 호출 ← **Controller 프로세스 시작**

---

##  **관계 다이어그램**

```
┌─────────────────────────────────────────────────────────────┐
│ github.com/openziti/fablab (프레임워크)                       │
├─────────────────────────────────────────────────────────────┤
│  ? model.ComponentType 인터페이스 정의                        │
│  ? model.FileStagingComponent 인터페이스 정의                 │
│  ? model.ServerComponent 인터페이스 정의                      │
│  ? Lifecycle 관리 (Express, Build, Sync, Activate)          │
│  ? SSH, rsync, Terraform 통합                               │
└─────────────────────────────────────────────────────────────┘
                           ▲
                           │ import
                           │
┌─────────────────────────────────────────────────────────────┐
│ github.com/openziti/ziti-test (실제 구현)                     │
├─────────────────────────────────────────────────────────────┤
│  zitilib/controller.go:                                     │
│    type ControllerType struct { ... }                       │
│    func (c *ControllerType) StageFiles(...) error { ... }   │
│    func (c *ControllerType) Start(...) error { ... }        │
│    func (c *ControllerType) Stop(...) error { ... }         │
│                                                              │
│  models/simple/main.go:                                     │
│    m := &model.Model{                                       │
│        Components: {                                        │
│            "controller": {                                  │
│                Type: &zitilib.ControllerType{...}           │
│            }                                                │
│        }                                                    │
│    }                                                        │
│    fablab.InitModel(m)                                     │
│    fablab.Run()                                            │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼ go build
                           
              ┌─────────────────────────────┐
              │ fablab-simple (실행 파일)     │
              ├─────────────────────────────┤
              │ ? fablab 프레임워크 포함       │
              │ ? ControllerType 구현 포함    │
              │ ? Model 정의 포함            │
              └─────────────────────────────┘
```

---

##  **요약**

| 항목 | fablab (프레임워크) | ziti-test (구현 프로젝트) |
|------|---------------------|-------------------------|
| **역할** | 인터페이스 정의, Lifecycle 관리 | 인터페이스 구현, 비즈니스 로직 |
| **제공** | `ComponentType` 인터페이스 | `ControllerType` struct 구현 |
| **코드 위치** | `github.com/openziti/fablab` | `github.com/openziti/ziti-test` |
| **의존성** | - | `import "github.com/openziti/fablab"` |
| **빌드 결과** | 라이브러리 (import됨) | 실행 파일 (`fablab-simple`) |
| **예시** | `model.FileStagingComponent` | `func StageFiles(...) { 실제 로직 }` |

**결론**: fablab은 **요리를 위한 주방(프레임워크)**이고, ziti-test는 **실제 요리(Controller 구현)**를 하는 프로젝트입니다. 주방(fablab)은 도구와 조리법(인터페이스)을 제공하고, 요리사(개발자)는 이를 사용해 실제 요리(Controller)를 만듭니다! 



#  **Router 설치 과정 세부 정리**

Router는 Controller와 동일한 4단계 Lifecycle을 거치지만, **Controller에 대한 의존성**과 **등록(Enrollment) 과정**이라는 중요한 차이점이 있습니다.

---

##  **Router vs Controller 차이점 요약**

| 항목 | Controller | Router |
|------|-----------|--------|
| **역할** | 네트워크 중앙 제어 | 데이터 전달 (Edge/Fabric) |
| **의존성** | 독립적 | Controller 필요    |
| **개수** | 일반적으로 1개 (HA 시 3개) | 다수 (수십~수백 개) |
| **등록 과정** | 없음 | Controller에 등록 필요    |
| **Init 단계** | 선택적 | 필수 (등록 처리)    |
| **Scale-out** | 제한적 | 자유로운 확장    |
| **설정 의존** | 독립적 | Controller 주소 필요    |

---

##  **전체 설치 Lifecycle**

Router는 Controller와 동일하게 `fablab up` 실행 시 4단계를 거칩니다:

```bash
fablab up
```

```
Express (Infrastructure) ───? Build (Configuration) ───? 
  Sync (Distribution) ───? Activate (Activation + Init)
```

하지만 **Activation 단계에서 추가로 Init이 필요**합니다.

---

##  **단계 1: Express (Infrastructure)**

### Controller와 동일

Router용 VM을 Terraform으로 생성합니다. Controller와 차이가 없습니다.

```yaml
# Model 정의 예시 (별도 프로젝트)
regions:
  us-east-1:
    hosts:
      ctrl01:         # Controller 호스트
        components:
          controller:
            type: ControllerType
      
      router01:       # Router 호스트 
        components:
          router:
            type: RouterType
      
      router02:       # Router 호스트 2 
        components:
          router:
            type: RouterType
```

**결과**:
- ? Controller VM 생성: `54.123.45.67`
- ? Router01 VM 생성: `54.123.45.68` 
- ? Router02 VM 생성: `54.123.45.69` 

---

##  **단계 2: Build (Configuration - Router 바이너리 준비)**

### Router의 StageFiles 구현

Router는 Controller와 달리 **Controller 주소 정보**가 필요합니다.

**zitilib/router.go** (별도 프로젝트 구현 예시):

```go
package zitilib

import (
    "fmt"
    "path/filepath"
    "github.com/openziti/fablab/kernel/model"
)

type RouterType struct {
    Version        string
    BinaryPath     string
    Mode           string  // "edge" or "fabric" 
    ControllerHost *model.Host  // Controller 참조 
}

// FileStagingComponent 구현
func (r *RouterType) StageFiles(run model.Run, comp *model.Component) error {
    // 1. Router 바이너리 복사
    binDest := filepath.Join(model.KitBuild(), "bin", "ziti-router")
    if err := copyFile(r.BinaryPath, binDest); err != nil {
        return fmt.Errorf("failed to copy router binary: %w", err)
    }
    
    // 2. Controller 주소 가져오기 
    ctrlComponent := r.ControllerHost.Components["controller"]
    ctrlAddr := fmt.Sprintf("tls://%s:6262", r.ControllerHost.PublicIp)
    
    // 3. Router 설정 파일 생성 (Controller 주소 포함) 
    configDest := filepath.Join(model.KitBuild(), "cfg", 
        fmt.Sprintf("router-%s.yml", comp.Id))
    
    config := fmt.Sprintf(`
v: 3
identity:
  cert: /home/ubuntu/fablab/pki/routers/%s/client.cert
  server_cert: /home/ubuntu/fablab/pki/routers/%s/server.cert
  key: /home/ubuntu/fablab/pki/routers/%s/server.key
  ca: /home/ubuntu/fablab/pki/ca/ca.cert

ctrl:
  endpoint: %s    # ← Controller 주소 

link:
  dialers:
    - binding: transport
  listeners:
    - binding: transport
      bind: tls:0.0.0.0:6000
      advertise: tls:%s:6000

listeners:
  - binding: edge
    address: tls:0.0.0.0:3022
    options:
      advertise: %s:3022
`, comp.Id, comp.Id, comp.Id, ctrlAddr, comp.Host.PublicIp, comp.Host.PublicIp)
    
    if err := writeFile(configDest, []byte(config)); err != nil {
        return fmt.Errorf("failed to write router config: %w", err)
    }
    
    // 4. 시작 스크립트 생성
    scriptDest := filepath.Join(model.KitBuild(), "scripts", 
        fmt.Sprintf("start-router-%s.sh", comp.Id))
    script := fmt.Sprintf(`#!/bin/bash
/home/ubuntu/fablab/bin/ziti-router run \
  /home/ubuntu/fablab/cfg/router-%s.yml
`, comp.Id)
    
    if err := writeFile(scriptDest, []byte(script)); err != nil {
        return fmt.Errorf("failed to write start script: %w", err)
    }
    
    // 5. PKI 생성 (Controller CA 사용) 
    pkiDest := filepath.Join(model.KitBuild(), "pki", "routers", comp.Id)
    if err := generateRouterPKI(comp, r.ControllerHost, pkiDest); err != nil {
        return fmt.Errorf("failed to generate PKI: %w", err)
    }
    
    return nil
}
```

### 로컬 Staging 디렉토리 (Router용)

```
~/.fablab/instances/<instance-id>/kit/build/
├── bin/
│   ├── controller          # Controller 바이너리
│   ├── ziti-router*        # Router 바이너리 
│   └── ziti
├── cfg/
│   ├── controller.yml
│   ├── router-router01.yml # Router01 설정 
│   └── router-router02.yml # Router02 설정 
├── scripts/
│   ├── start-controller.sh
│   ├── start-router-router01.sh* 
│   └── start-router-router02.sh* 
└── pki/
    ├── ca/
    ├── controller/
    └── routers/            # Router PKI 
        ├── router01/
        │   ├── client.cert
        │   ├── server.cert
        │   └── server.key
        └── router02/
            ├── client.cert
            ├── server.cert
            └── server.key
```

**핵심 차이점**:
-  **Controller 주소**: Router 설정에 `ctrl.endpoint` 포함
-  **개별 설정**: 각 Router마다 별도의 설정 파일
-  **PKI 의존성**: Controller CA를 사용하여 인증서 생성

---

##  **단계 3: Sync (Distribution)**

### Controller와 동일하지만 더 많은 파일

```bash
# 로컬 → Router01 VM
rsync -avz --delete \
  ~/.fablab/instances/my-instance/kit/build/ \
  ubuntu@54.123.45.68:/home/ubuntu/fablab/

# 로컬 → Router02 VM
rsync -avz --delete \
  ~/.fablab/instances/my-instance/kit/build/ \
  ubuntu@54.123.45.69:/home/ubuntu/fablab/
```

### InitializeHost 구현

```go
func (r *RouterType) InitializeHost(run model.Run, comp *model.Component) error {
    host := comp.GetHost()
    
    cmds := []string{
        "chmod +x /home/ubuntu/fablab/bin/ziti-router",
        fmt.Sprintf("chmod +x /home/ubuntu/fablab/scripts/start-router-%s.sh", comp.Id),
        "mkdir -p /home/ubuntu/fablab/data/router",
        "mkdir -p /home/ubuntu/fablab/logs",
        
        // Router 전용: 네트워크 설정 
        "sudo sysctl -w net.ipv4.ip_forward=1",
        "sudo sysctl -w net.core.somaxconn=4096",
    }
    
    for _, cmd := range cmds {
        if _, err := host.ExecLogged(cmd); err != nil {
            return fmt.Errorf("failed to exec %s: %w", cmd, err)
        }
    }
    
    return nil
}
```

**결과**:
-  Router 바이너리 및 설정 파일이 각 Router VM에 전송됨
-  실행 권한 설정 완료
-  Label 상태: `Distributed`

---

##  **단계 4: Activate (Activation + Init)**

### 4-1. 컴포넌트 시작 순서 제어 

Router는 Controller에 의존하므로, **순서가 중요**합니다:

```go
// Model 정의 (별도 프로젝트)
func (m *MyModel) Build() error {
    // Activation 단계 정의
    m.AddActivationActions(
        "start-controller",   // 1. Controller 먼저 시작 
        "init-routers",       // 2. Router 등록 (Init) 
        "start-routers",      // 3. Router 프로세스 시작 
    )
    return nil
}

// Action 정의
m.Actions = map[string]model.ActionBinder{
    "start-controller": model.Bind(
        component.StartInParallel("#controller", 1),
    ),
    
    "init-routers": model.Bind(
        component.ExecInParallel("#router", 5, "init"), //  Init 액션
    ),
    
    "start-routers": model.Bind(
        component.StartInParallel("#router", 5),
    ),
}
```

### 4-2. Router Init 구현 (Controller 등록) 

```86:95:kernel/model/component.go
// A InitializingComponent can run some configuration on the host as part of the activation phase.
// Init isn't called explicitly as it often has dependencies on other components. However, by
// implementing this interface, the action will be made available, without requiring explicit
// registration
type InitializingComponent interface {
	ComponentType

	// Init needs to be called explicitly
	Init(r Run, c *Component) error
}
```

**RouterType.Init 구현** (별도 프로젝트):

```go
// InitializingComponent 인터페이스 구현 
func (r *RouterType) Init(run model.Run, comp *model.Component) error {
    host := comp.GetHost()
    
    logrus.Infof(" Enrolling router %s to controller...", comp.Id)
    
    // 1. Controller Management API 주소
    ctrlMgmtAddr := fmt.Sprintf("https://%s:1280", r.ControllerHost.PublicIp)
    
    // 2. Controller에 Router 생성 요청
    createRouterCmd := fmt.Sprintf(`
/home/ubuntu/fablab/bin/ziti edge create edge-router %s \
  -a "public" \
  -o /home/ubuntu/fablab/data/%s.jwt \
  --jwt-output-file /home/ubuntu/fablab/data/%s.jwt
`, comp.Id, comp.Id, comp.Id)
    
    // Controller에서 실행 (SSH를 통해)
    if _, err := r.ControllerHost.ExecLogged(createRouterCmd); err != nil {
        return fmt.Errorf("failed to create router in controller: %w", err)
    }
    
    // 3. JWT 파일을 Router 호스트로 복사
    jwtContent, err := r.ControllerHost.ExecLogged(
        fmt.Sprintf("cat /home/ubuntu/fablab/data/%s.jwt", comp.Id),
    )
    if err != nil {
        return fmt.Errorf("failed to read JWT: %w", err)
    }
    
    // Router 호스트에 JWT 저장
    jwtPath := fmt.Sprintf("/home/ubuntu/fablab/data/%s.jwt", comp.Id)
    if err := host.SendData([]byte(jwtContent), jwtPath); err != nil {
        return fmt.Errorf("failed to send JWT: %w", err)
    }
    
    // 4. Router Enrollment (등록 완료)
    enrollCmd := fmt.Sprintf(`
/home/ubuntu/fablab/bin/ziti-router enroll \
  /home/ubuntu/fablab/cfg/router-%s.yml \
  --jwt %s
`, comp.Id, jwtPath)
    
    if output, err := host.ExecLogged(enrollCmd); err != nil {
        logrus.Errorf("Enrollment output: %s", output)
        return fmt.Errorf("failed to enroll router: %w", err)
    }
    
    logrus.Infof("? Router %s enrolled successfully", comp.Id)
    return nil
}
```

### 4-3. Router Start 구현

Init 완료 후 Router 프로세스를 시작합니다:

```go
// ServerComponent 인터페이스 구현
func (r *RouterType) Start(run model.Run, comp *model.Component) error {
    host := comp.GetHost()
    
    // 1. 기존 프로세스 종료
    host.KillProcesses("-9", func(line string) bool {
        return strings.Contains(line, "ziti-router")
    })
    
    // 2. nohup으로 백그라운드 실행
    startCmd := fmt.Sprintf(
        "nohup /home/ubuntu/fablab/scripts/start-router-%s.sh "+
        "> /home/ubuntu/fablab/logs/router-%s.log 2>&1 &",
        comp.Id, comp.Id,
    )
    
    if _, err := host.ExecLogged(startCmd); err != nil {
        return fmt.Errorf("failed to start router: %w", err)
    }
    
    // 3. 시작 확인
    time.Sleep(5 * time.Second)  // Router는 Controller 연결 대기 필요
    
    // 4. Health Check
    running, err := r.IsRunning(run, comp)
    if err != nil || !running {
        return fmt.Errorf("router failed to start")
    }
    
    // 5. Controller 연결 확인 
    if err := r.verifyControllerConnection(run, comp); err != nil {
        return fmt.Errorf("router started but failed to connect to controller: %w", err)
    }
    
    logrus.Infof("? Router %s started and connected to controller", comp.Id)
    return nil
}

// Controller 연결 확인 
func (r *RouterType) verifyControllerConnection(run model.Run, comp *model.Component) error {
    // Controller에서 Router 상태 확인
    checkCmd := fmt.Sprintf(
        "/home/ubuntu/fablab/bin/ziti edge list edge-routers 'name=\"%s\"' -j",
        comp.Id,
    )
    
    output, err := r.ControllerHost.ExecLogged(checkCmd)
    if err != nil {
        return err
    }
    
    // JSON 파싱하여 online 상태 확인
    if strings.Contains(output, `"online":true`) {
        return nil
    }
    
    return fmt.Errorf("router is not online in controller")
}
```

---

##  **Router 설치 전체 프로세스 다이어그램**

```
┌─────────────────────────────────────────────────────────────┐
│ fablab up                                                     │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────────────────────────┐
│ 1. Express (Infrastructure)                                │
├────────────────────────────────────────────────────────────┤
│  ? Terraform: Controller VM 생성 (54.123.45.67)            │
│  ? Terraform: Router01 VM 생성 (54.123.45.68)              │
│  ? Terraform: Router02 VM 생성 (54.123.45.69)              │
│  → Label.State = Expressed                                 │
└────────────────┬───────────────────────────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────────────────────────┐
│ 2. Build (Configuration)                                   │
├────────────────────────────────────────────────────────────┤
│  Controller:                                               │
│    ? ControllerType.StageFiles()                          │
│      - controller 바이너리 복사                             │
│      - controller.yml 생성                                 │
│                                                            │
│  Router01 & Router02:                                      │
│    ? RouterType.StageFiles()                              │
│      - ziti-router 바이너리 복사                           │
│      - router-router01.yml 생성 (Controller 주소 포함)      │
│      - router-router02.yml 생성 (Controller 주소 포함)      │
│      - PKI 생성 (Controller CA 사용)                        │
│  → Label.State = Configured                               │
└────────────────┬───────────────────────────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────────────────────────┐
│ 3. Sync (Distribution)                                     │
├────────────────────────────────────────────────────────────┤
│  ? rsync: 로컬 → Controller VM                             │
│  ? rsync: 로컬 → Router01 VM                               │
│  ? rsync: 로컬 → Router02 VM                               │
│  ? InitializeHost: 권한 설정, 디렉토리 생성                  │
│  → Label.State = Distributed                              │
└────────────────┬───────────────────────────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────────────────────────┐
│ 4. Activate (Activation)                                   │
├────────────────────────────────────────────────────────────┤
│  Step 1: start-controller                                 │
│    ? ControllerType.Start()                               │
│      - Controller 프로세스 시작                             │
│      - Health Check                                        │
│                                                            │
│  Step 2: init-routers                                      │
│    ? RouterType.Init() (Router01)                         │
│      - Controller API로 Router 생성                         │
│      - JWT 토큰 생성 및 전송                                │
│      - Router Enrollment                                   │
│    ? RouterType.Init() (Router02)                         │
│      - 동일한 프로세스                                       │
│                                                            │
│  Step 3: start-routers                                     │
│    ? RouterType.Start() (Router01)                        │
│      - Router 프로세스 시작                                 │
│      - Controller 연결 확인                                 │
│    ? RouterType.Start() (Router02)                        │
│      - Router 프로세스 시작                                 │
│      - Controller 연결 확인                                 │
│                                                            │
│  → Label.State = Activated                                │
└────────────────────────────────────────────────────────────┘
```

---

##  **Router만의 특징 상세**

### 1. **InitializingComponent 인터페이스 활용** 

Controller는 Init이 필요 없지만, Router는 필수입니다:

```go
// Controller
type ControllerType struct { ... }
// Init 구현 안 함 - InitializingComponent 인터페이스 미구현

// Router
type RouterType struct { ... }
// Init 구현 필수 - InitializingComponent 인터페이스 구현 
func (r *RouterType) Init(run model.Run, c *Component) error {
    // Controller에 등록하는 로직
}
```

### 2. **Component 선택자 활용** 

fablab은 강력한 Component 선택 기능을 제공합니다:

```bash
# 모든 Router 시작
fablab start "#router"

# 특정 Router만 시작
fablab start "router01"

# Tag 기반 선택
fablab start "@edge-router"

# 정규식 선택
fablab start "router*"
```

**Model 정의 예시**:
```go
m := &model.Model{
    Regions: model.Regions{
        "us-east-1": {
            Hosts: model.Hosts{
                "router01": {
                    Tags: []string{"edge-router", "public"},
                    Components: model.Components{
                        "router": {
                            Type: &RouterType{Mode: "edge"},
                        },
                    },
                },
                "router02": {
                    Tags: []string{"fabric-router", "internal"},
                    Components: model.Components{
                        "router": {
                            Type: &RouterType{Mode: "fabric"},
                        },
                    },
                },
            },
        },
    },
}
```

### 3. **Scale-out 지원** 

Router는 쉽게 확장 가능합니다:

```go
// 10개의 Router를 자동으로 생성
m.AddScaleFactory("router-scale", 10, func(index int) *model.Host {
    return &model.Host{
        InstanceType: "t3.small",
        Tags: []string{"edge-router"},
        Components: model.Components{
            "router": {
                Type: &RouterType{
                    Version:        "v0.30.0",
                    Mode:           "edge",
                    ControllerHost: ctrlHost,
                },
            },
        },
    }
})
```

### 4. **병렬 처리** 

여러 Router를 병렬로 처리하여 배포 시간 단축:

```go
// 10개의 Router를 동시에 초기화
m.AddActivationAction("init-routers")

m.Actions["init-routers"] = model.Bind(
    component.ExecInParallel("#router", 10, "init"),  // 동시성 10
)
```

```34:41:kernel/lib/actions/component/start.go
func (start *start) Execute(run model.Run) error {
	return run.GetModel().ForEachComponent(start.componentSpec, start.concurrency, func(c *model.Component) error {
		if startable, ok := c.Type.(ServerComponent); ok {
			return startable.Start(run, c)
		}
		return nil
	})
}
```

---

##  **Controller vs Router 설치 비교표**

| 단계 | Controller | Router |
|------|-----------|--------|
| **Express** | VM 생성 | VM 생성 (동일) |
| **Build** | - 바이너리 복사<br>- 독립적 설정 생성 | - 바이너리 복사<br>- **Controller 주소 포함** 설정 생성 <br>- **Controller CA 사용** PKI 생성  |
| **Sync** | 파일 전송, 권한 설정 | 파일 전송, 권한 설정, **네트워크 설정**  (동일) |
| **Activate** | 1. Controller 시작<br>2. Health Check | 1. Controller 시작 (먼저) <br>2. **Router Init (등록)** <br>3. Router 시작<br>4. **Controller 연결 확인**  |
| **Init 단계** | 없음 | **필수** (Controller 등록)  |
| **의존성** | 없음 | **Controller 필수**  |
| **개수** | 1~3개 | 무제한 확장 가능  |

---

##  **실행 예시**

### 전체 배포

```bash
# Controller + Router 전체 배포
fablab up

# 실행 순서:
# 1. Express: Controller VM + Router VM들 생성
# 2. Build: Controller & Router 바이너리, 설정 준비
# 3. Sync: 모든 VM에 파일 전송
# 4. Activate:
#    - start-controller → Controller 시작
#    - init-routers → 모든 Router를 Controller에 등록 
#    - start-routers → 모든 Router 시작 
```

### Router만 재시작

```bash
# 모든 Router 재시작
fablab restart "#router"

# 특정 Router만 재시작
fablab restart "router01"

# Edge Router만 재시작
fablab restart "@edge-router"
```

### Router 추가 배포

```bash
# 새로운 Router 추가 (Model 수정 후)
fablab refresh  # Build → Sync → Activate만 재실행

# 또는
fablab exec init-routers   # 새 Router만 등록
fablab start "router03"    # 새 Router만 시작
```

---

##  **요약**

Router 설치는 Controller와 **거의 동일한 프로세스**를 거치지만, 다음의 **추가 단계**가 필요합니다:

1.  **Controller 주소 의존성**: Build 단계에서 Controller 주소를 설정에 포함
2.  **Init 단계 필수**: Activate 단계에서 Controller에 등록 (Enrollment)
3.  **순서 제어**: Controller → Router Init → Router Start 순서 보장
4.  **연결 확인**: Router 시작 후 Controller와의 연결 상태 검증
5.  **Scale-out 지원**: 여러 Router를 병렬로 배포 가능

**핵심**: Router는 **Controller에 의존하는 분산 컴포넌트**로, `InitializingComponent` 인터페이스를 통해 등록 과정을 처리하고, fablab의 순서 제어 메커니즘을 활용하여 안전하게 배포됩니다! 

# Fablab Model 인스턴스 모니터링 및 관리 

## Fablab의 Model 인스턴스 모니터링 및 관리 기능 조사 결과

### 1. 인스턴스 생성 및 상태 관리

#### 인스턴스 생성
```81:90:cmd/fablab/subcmd/create.go
	instanceId, err := model.NewInstance(self.Name, self.WorkingDir, self.Executable)
	if err != nil {
		return errors.Wrapf(err, "unable to create instance of model %v, exiting", model.GetModel().Id)
	}

	logrus.Infof("allocated new instance [%v] for model %v", instanceId, model.GetModel().GetId())

	if err := model.CreateLabel(instanceId, self.Bindings); err != nil {
		return errors.Wrapf(err, "unable to create instance label [%s]", instanceId)
	}
```

#### 상태 추적 시스템
fablab은 인스턴스 상태를 `Label` 구조체로 관리하며, 다음과 같은 라이프사이클 상태를 추적합니다:

```159:169:kernel/model/label.go
type InstanceState int

const (
	Created InstanceState = iota
	Expressed
	Configured
	Distributed
	Activated
	Operating
	Disposed
)
```

각 라이프사이클 단계마다 상태가 자동으로 업데이트됩니다:

```1208:1218:kernel/model/model.go
func (m *Model) Operate(run Run) error {
	for _, stage := range m.Operation {
		if err := stage.Execute(run); err != nil {
			return fmt.Errorf("error operating (%w)", err)
		}
	}
	run.GetLabel().State = Operating
	if err := run.GetLabel().Save(); err != nil {
		return fmt.Errorf("error updating instance label (%w)", err)
	}
	return nil
}
```

### 2. 모니터링 기능

#### 메트릭 수집 시스템
Model 구조체는 `MetricsHandlers`를 통해 메트릭을 처리합니다:

```1234:1238:kernel/model/model.go
func (m *Model) AcceptHostMetrics(host *Host, event *MetricsEvent) {
	for _, handler := range m.MetricsHandlers {
		handler.AcceptHostMetrics(host, event)
	}
}
```

#### SAR 기반 시스템 메트릭 수집
- 실시간 스트리밍: `StreamSarMetrics`를 통해 호스트의 CPU, 메모리, 프로세스 큐 메트릭을 주기적으로 수집합니다:

```74:109:kernel/lib/runlevel/5_operation/stream_sar_metrics.go
func (s *streamSarMetrics) runSar(ssh libssh.SshConfigFactory) {
	defer func() {
		logrus.Debugf("runSar complete")
	}()

	for !s.closed.Load() {
		if err := s.reportMetrics(ssh); err != nil {
			return
		}
	}
}

func (s *streamSarMetrics) reportMetrics(ssh libssh.SshConfigFactory) error {
	log := pfxlog.Logger().WithField("addr", ssh.Address())
	sarCmd := fmt.Sprintf("sar -u -r -q %d %d", s.intervalSeconds, s.reportIntervalCount)
	output, err := libssh.RemoteExec(ssh, sarCmd)
	if err != nil {
		log.WithError(err).Warnf("sar exited: %s", output)
		return err
	}

	summary, err := lib.SummarizeSar([]byte(output))
	if err != nil {
		log.WithError(err).Errorf("sar summary failed [%s]", output)
		return err
	}

	events := summary.ToMetricsEvents()
	m := s.host.GetRegion().GetModel()
	for _, event := range events {
		m.AcceptHostMetrics(s.host, event)
	}

	log.Infof("%v sar metrics events reported", len(events))
	return nil
}
```

#### 메트릭 핸들러 유형
1. DataMetricsWriter: 호스트의 Data 필드에 메트릭 저장

```229:241:kernel/model/metrics.go
func (DataMetricsWriter) AcceptHostMetrics(host *Host, event *MetricsEvent) {
	var metricsSlice []map[string]interface{}
	val, found := host.Data["metrics"]
	if found {
		metricsSlice = val.([]map[string]interface{})
	}
	metricsMap := map[string]interface{}{}
	metricsMap["timestamp_ms"] = fmt.Sprintf("%v", timeutil.TimeToMilliseconds(event.Timestamp))
	for name, val := range event.Metrics {
		metricsMap[name] = val
	}
	host.Data["metrics"] = append(metricsSlice, metricsMap)
}
```

2. StdOutMetricsWriter: 콘솔에 메트릭 출력

```246:251:kernel/model/metrics.go
func (StdOutMetricsWriter) AcceptHostMetrics(host *Host, event *MetricsEvent) {
	fmt.Printf("metrics event - host %v at timestamp: %v\n", host.GetId(), event.Timestamp)
	for k, v := range event.Metrics {
		fmt.Printf("\t%v = %v\n", k, v)
	}
}
```

3. InfluxDB 메트릭 리포터: InfluxDB로 메트릭 전송

```97:102:kernel/lib/runlevel/5_operation/influxdb.go
func (reporter *influxReporter) AcceptHostMetrics(host *model.Host, event *model.MetricsEvent) {
	reporter.metricsChan <- &hostMetricsEvent{
		host:  host,
		event: event,
	}
}
```

### 3. 관리 기능

#### 인스턴스 목록 조회
```70:98:cmd/fablab/subcmd/list.go
func listInstances(_ *cobra.Command, _ []string) {
	cfg := model.GetConfig()
	activeInstanceId := cfg.Default

	var instanceIds []string
	for k := range cfg.Instances {
		instanceIds = append(instanceIds, k)
	}

	sort.Strings(instanceIds)

	fmt.Println()
	fmt.Printf("[%d] instances:\n\n", len(instanceIds))
	for _, instanceId := range instanceIds {
		idLabel := instanceId
		if instanceId == activeInstanceId {
			idLabel += "*"
		}
		instanceConfig := cfg.Instances[instanceId]
		if l, err := instanceConfig.LoadLabel(); err == nil {
			fmt.Printf("%-12s %-24s [%s]\n", idLabel, l.Model, l.State)
		} else {
			fmt.Printf("%-12s %s\n", idLabel, err)
		}
	}
	if len(instanceIds) > 0 {
		fmt.Println()
	}
}
```

#### 인스턴스 상태 확인
```37:51:cmd/fablab/subcmd/status.go
func status(_ *cobra.Command, _ []string) {
	if err := model.Bootstrap(); err != nil {
		logrus.Fatal("unable to bootstrap (%w)", err)
	}

	l := model.GetLabel()
	if l == nil {
		fmt.Printf("%-20s no label\n", "Label")
	} else {
		fmt.Printf("%-20s\n", "Label")
		fmt.Printf("%-20s %s\n", "  Model", l.Model)
		fmt.Printf("%-20s %s\n", "  State", l.State)
	}
	fmt.Println()
}
```

#### 라이프사이클 관리
Operation 단계에서 다양한 모니터링 및 관리 작업을 수행할 수 있습니다:

```1208:1218:kernel/model/model.go
func (m *Model) Operate(run Run) error {
	for _, stage := range m.Operation {
		if err := stage.Execute(run); err != nil {
			return fmt.Errorf("error operating (%w)", err)
		}
	}
	run.GetLabel().State = Operating
	if err := run.GetLabel().Save(); err != nil {
		return fmt.Errorf("error updating instance label (%w)", err)
	}
	return nil
}
```

#### 추가 관리 기능
- Timer: 지정된 시간 동안 대기 후 종료 신호 전송
- Retrieve: 원격 호스트에서 파일 수집 및 포렌식 데이터 관리
- Process 관리: 호스트에서 프로세스 찾기 및 종료 기능 (`FindProcesses`, `KillProcesses`)

### 4. 주요 특징

1. 상태 지속성: 인스턴스 상태가 `fablab.yml` 파일에 저장되어 관리됩니다
2. 확장 가능한 메트릭 시스템: `MetricsHandler` 인터페이스를 통해 다양한 메트릭 저장소로 확장 가능
3. 원격 모니터링: SSH를 통해 원격 호스트의 시스템 메트릭을 실시간으로 수집
4. 라이프사이클 추적: 7단계(Created → Expressed → Configured → Distributed → Activated → Operating → Disposed) 상태 관리

fablab은 인스턴스 생성 후 상태 추적, 메트릭 수집, 원격 모니터링, 라이프사이클 관리를 제공하는 모니터링 및 관리 시스템을 제공합니다.



# Fablab cli를 완전 제거

CLI를 완전히 배제한 재설계 제안을 정리했습니다. 추가 설계 문서 작성 중입니다.

## Fablab CLI 완전 배제 재설계 제안

fablab의 CLI 구조를 제거하고 재설계하는 방안을 정리했습니다. 구현 없이 설계와 방향에 집중했습니다.

### 생성된 문서

1. **`docs/cli_free_redesign.md`** - 전체 재설계 제안서 (주요 문서)
2. **`docs/architecture_after_cli_removal.md`** - 아키텍처 상세 설계
3. **`docs/redesign_summary.md`** - 요약 문서

### 재설계 핵심 개념

#### 설계 철학

fablab을 **CLI 도구**가 아닌 **프로그래밍 프레임워크/플랫폼**으로 재정의:

1. 코드 우선: 모든 작업은 Go 코드로 구현
2. 프로그래밍 가능성: 라이브러리/SDK로 직접 사용
3. 구성 중심: 선언적 구성 파일 (Infrastructure as Code)
4. API 중심: 모든 기능을 API로 노출
5. 통합 우선: 다양한 환경과 통합

#### 새로운 아키텍처

```
사용 인터페이스 레이어
├── Go SDK (프로그래밍)
├── REST/gRPC API (원격 접근)
├── 구성 파일 (IaC)
└── 웹 대시보드 (UI)
         ↓
SDK 레이어
├── Client API
├── Model Builder
└── Instance Manager
         ↓
Core Engine
├── Runtime (다중 인스턴스 지원)
├── Executor (작업 실행)
└── Scheduler (스케줄링)
         ↓
Kernel Layer (기존 유지)
├── Model
├── Lifecycle
└── Lib
```

### 사용 방법

#### 방법 1: Go 코드에서 직접 사용
```go
client := fablab.NewClient(ctx, fablab.Config{})
instance, _ := client.Instances().Create(ctx, config)
instance.Up(ctx)
```

#### 방법 2: 구성 파일 기반 (IaC)
```yaml
# fablab.yml
apiVersion: fablab.io/v1
kind: Model
spec:
  regions: [...]
  lifecycle: {...}
```

#### 방법 3: REST API
```bash
curl -X POST http://fablab-server/api/v1/instances/prod/lifecycle/up
```

#### 방법 4: GitOps/CI/CD 통합
```yaml
# GitHub Actions
- uses: openziti/fablab-action@v1
- run: fablab instance up production
```

### 주요 개선 사항

1. 전역 상태 제거: 컨텍스트 기반 설계로 다중 인스턴스 지원
2. SDK 중심: Go SDK로 프로그래밍 가능
3. 구성 파일 지원: YAML/JSON로 선언적 정의
4. API 서버: REST/gRPC로 다양한 클라이언트 지원
5. 통합 포인트: IDE, CI/CD, 웹 대시보드 통합

### 핵심 인터페이스 설계

- **Client**: 메인 진입점 (모델/인스턴스/구성 관리)
- **Instance**: 라이프사이클 관리 (Up/Down, 개별 단계)
- **ConfigManager**: 구성 파일 로드/저장/검증
- **Runtime**: 다중 인스턴스 런타임 관리

### 비교: Before vs After

**Before (CLI 중심):**
```bash
fablab create my-instance
fablab up
fablab express
fablab operate
```

**After (코드/구성 중심):**
```go
// 프로그래밍 방식
client := fablab.NewClient(...)
instance := client.Instances().Create(...)
instance.Up(...)
```

또는

```yaml
# 구성 파일 방식
# fablab.yml 정의 후
fablab run fablab.yml
```

### 장점

1. 프로그래밍 가능성: Go 코드로 직접 통합
2. 구성 중심: GitOps와 IaC 지원
3. API 우선: 다양한 클라이언트 구현 가능
4. 통합 용이: CI/CD, IDE, 웹 등과 통합
5. 확장성: 플러그인 및 확장 구조

### 마이그레이션 전략

4단계 점진적 전환:
- Phase 1: SDK 레이어 추가
- Phase 2: 구성 파일 지원
- Phase 3: API 서버 추가
- Phase 4: CLI 제거 (선택적)

자세한 내용은 `docs/cli_free_redesign.md`를 참고하세요. 이 제안은 fablab을 CLI 도구에서 프로그래밍 프레임워크로 재정의합니다.

# model.Model Action 구조와 사용 방식  

## Model.Model의 Action 시스템 조사 결과

model.Model의 Action 구조와 사용 방식을 정리했습니다.

### 생성된 문서

- **`docs/action_investigation.md`** - Action 시스템 조사 문서

### 핵심 발견 사항

#### 1. Action 인터페이스

```843:851:kernel/model/model.go
type Action interface {
	Execute(run Run) error
}

type ActionFunc func(run Run) error

func (f ActionFunc) Execute(run Run) error {
	return f(run)
}
```

- 단일 메서드 인터페이스: `Execute(run Run) error`
- 함수 타입으로도 구현 가능 (`ActionFunc`)

#### 2. ActionBinder 패턴

```830:841:kernel/model/model.go
type ActionBinder func(m *Model) Action
type ActionBinders map[string]ActionBinder

func Bind(action Action) ActionBinder {
	return func(m *Model) Action {
		return action
	}
}

func BindF(f func(run Run) error) ActionBinder {
	return Bind(ActionFunc(f))
}
```

- 지연 바인딩: Bootstrap 시점에 Action 생성
- 모델에 따라 다른 Action 반환 가능

#### 3. Model의 Action 저장 구조

```222:232:kernel/model/model.go
	Actions             map[string]ActionBinder  // Action 바인더 (설정)
	// ...
	actions map[string]Action  // 실제 바인딩된 Action (실행)
```

- 두 단계 저장:
  1. `Actions`: ActionBinder 저장 (설정 시)
  2. `actions`: 실제 Action 저장 (Bootstrap 시 바인딩)

#### 4. Bootstrap 시 Action 바인딩

```106:110:kernel/model/bootstrap.go
		model.actions = make(map[string]Action)
		for name, binder := range model.Actions {
			model.actions[name] = binder(model)
			logrus.Debugf("bound action [%s]", name)
		}
```

- Bootstrap 단계에서 ActionBinder를 실행하여 실제 Action 생성
- 생성된 Action을 내부 맵에 저장

#### 5. Action 등록 방법

```1066:1076:kernel/model/model.go
func (m *Model) AddActionBinder(actionName string, action ActionBinder) {
	m.Actions[actionName] = action
}

func (m *Model) AddAction(actionName string, action Action) {
	m.Actions[actionName] = Bind(action)
}

func (m *Model) AddActionF(actionName string, action ActionFunc) {
	m.Actions[actionName] = Bind(action)
}
```

- `AddActionBinder`: ActionBinder 직접 등록
- `AddAction`: Action 인터페이스 구현체 등록
- `AddActionF`: 함수로 Action 등록 (가장 간단)

#### 6. Action 실행 방법

**직접 실행:**
```42:59:kernel/model/selector.go
func (m *Model) Exec(r Run, actionNames ...string) error {
	var actions []Action
	for _, actionName := range actionNames {
		action, found := m.actions[actionName]
		if !found {
			return errors.Errorf("action %s not found", actionName)
		}
		actions = append(actions, action)
	}

	for idx, action := range actions {
		if err := action.Execute(r); err != nil {
			return fmt.Errorf("error executing action %s (%w)", actionNames[idx], err)
		}
	}

	return nil
}
```

**Stage로 변환:**
```1042:1064:kernel/model/model.go
func RunAction(action string) Action {
	return actionStage(action)
}

type actionStage string

func (stage actionStage) Execute(run Run) error {
	return stage.execute(run)
}

func (stage actionStage) execute(run Run) error {
	actionName := string(stage)
	m := run.GetModel()
	action, found := m.GetAction(actionName)
	if !found {
		return fmt.Errorf("no [%s] action", actionName)
	}
	figlet.FigletMini("action: " + actionName)
	if err := action.Execute(run); err != nil {
		return fmt.Errorf("error executing [%s] action (%w)", actionName, err)
	}
	return nil
}
```

#### 7. Action과 라이프사이클의 관계

```1101:1105:kernel/model/model.go
func (m *Model) AddActivationActions(actions ...string) {
	for _, action := range actions {
		m.AddActivationStage(actionStage(action))
	}
}
```

- Action은 라이프사이클 단계와 독립적으로 실행 가능
- 라이프사이클 단계에 포함시켜 자동 실행 가능

#### 8. Component Action vs Model Action

**Model Action:**
- 모델 레벨에서 정의
- 전체 모델에 대한 작업

**Component Action:**
```106:124:kernel/model/component.go
// A ComponentAction is an action execute in the context of a specific component
type ComponentAction interface {
	Execute(r Run, c *Component) error
}

// An ActionsComponent provides additional actions which can be executed using the ExecuteAction method
type ActionsComponent interface {
	ComponentType

	// GetActions returns the set of additional actions available on the component
	GetActions() map[string]ComponentAction
}
```

- 컴포넌트 컨텍스트에서 실행
- 컴포넌트별로 다른 Action 제공

### Action의 특징

1. 라이프사이클 독립성: 어떤 단계에서든 실행 가능
2. 모델 레벨 작업: 전체 모델에 대한 작업 수행
3. 재사용성: Workflow로 조합 가능
4. 확장성: 사용자가 커스텀 Action 정의 가능

### 사용 패턴

1. 함수로 정의: `AddActionF("name", func(run Run) error {...})`
2. 구조체로 정의: `AddAction("name", &MyAction{...})`
3. ActionBinder로 동적 생성: `AddActionBinder("name", func(m *Model) Action {...})`
4. 라이프사이클에 포함: `AddActivationActions("action1", "action2")`

자세한 내용은 `docs/action_investigation.md`를 참고하세요.



## `fablab.Run()`의 동작 방식

**매번 처음부터 한 번 실행하고 종료하는 방식**입니다.

### 코드 분석

```13:17:main.go
func Run() {
	if err := subcmd.RootCmd.Execute(); err != nil {
		logrus.WithError(err).Fatal("failure")
	}
}
```

`fablab.Run()`은 `cobra.Command`의 `Execute()`를 호출합니다. Cobra는 명령을 파싱하고 실행한 뒤 종료합니다.

### 실행 흐름

각 subcommand 실행 시:

```40:77:cmd/fablab/subcmd/exec.go
func runExec(_ *cobra.Command, args []string) {
	if err := model.Bootstrap(); err != nil {
		logrus.Fatalf("unable to bootstrap (%s)", err)
	}

	ctx, err := model.NewRun()
	if err != nil {
		logrus.WithError(err).Fatal("error initializing run")
	}

	m := model.GetModel()

	if !m.IsBound() {
		logrus.Fatalf("model not bound")
	}

	for _, binding := range execCmdBindings {
		if err := execCmdBind(m, binding); err != nil {
			logrus.Fatalf("error binding [%s] (%v)", binding, err)
		}
	}

	var actions []model.Action

	for _, name := range args {
		action, found := m.GetAction(name)
		if !found {
			logrus.Fatalf("no such action [%s]", name)
		}
		actions = append(actions, action)
	}

	for _, action := range actions {
		if err := action.Execute(ctx); err != nil {
			logrus.WithError(err).Fatalf("action failed [%+v]", action)
		}
	}
}
```

각 명령 실행 시:
1. `model.Bootstrap()` 호출 (모델 초기화)
2. `model.NewRun()` 호출 (새 실행 컨텍스트 생성)
3. 명령 실행
4. 종료

### 특수 케이스: exec-loop

`exec-loop`는 단일 명령 내에서 반복 실행합니다:

```97:112:cmd/fablab/subcmd/exec_loop.go
	for {
		iterationStart := time.Now()
		figlet.Figlet(fmt.Sprintf("ITERATION-%03d", iterations))
		for _, action := range actions {
			if err = action.Execute(ctx); err != nil {
				logrus.WithError(err).Fatalf("action failed [%+v]", action)
			}
		}
		if until.isDone() {
			pfxlog.Logger().Infof("finished after %v iteration(s) in %v", iterations, time.Since(start))
			return
		}
		pfxlog.Logger().Infof("iteration: %v, iteration time: %v, total time: %v",
			iterations, time.Since(iterationStart), time.Since(start))
		iterations++
	}
```

이 경우에도 하나의 명령으로 실행되고 종료됩니다.

### 요약

- `fablab.Run()`은 일회성 실행: 명령 실행 후 종료
- 각 subcommand는 독립 실행: 매번 Bootstrap과 NewRun 수행
- 대화형/서버 모드 아님: 계속 실행하며 명령을 대기하지 않음
- `exec-loop`는 단일 명령 내에서 반복 실행

따라서 매번 처음부터 실행하고 종료하는 전형적인 CLI 도구 동작 방식입니다.
