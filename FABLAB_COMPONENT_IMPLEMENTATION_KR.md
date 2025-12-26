# Fablab Component 구현 참조 문서

이 문서는 Fablab 프레임워크를 기반으로 Ziti Controller와 Router 컴포넌트를 구현한 레퍼런스 코드입니다. 이 코드는 `ComponentType` 및 관련 라이프사이클 인터페이스(`FileStagingComponent`, `HostInitializingComponent`, `ServerComponent`, `InitializingComponent`)가 어떻게 활용되는지 보여줍니다.

---

## 1. Ziti Controller 구현 (`zitilib/controller.go`)

Controller는 표준적인 4단계 라이프사이클(Express -> Build -> Sync -> Activate)을 따르며, 초기화(Init) 과정이 필요 없는 독립적인 컴포넌트입니다.

```go
package zitilib

import (
    "fmt"
    "path/filepath"
    "strings"
    "time"
    
    "github.com/openziti/fablab/kernel/model"
    "github.com/sirupsen/logrus"
)

// ControllerType은 Ziti Controller 컴포넌트의 구체적인 구현체입니다.
type ControllerType struct {
    Version      string
    BinaryPath   string  // 로컬에서 controller 바이너리 경로
}

// -----------------------------------------------------------------------------
// ComponentType 인터페이스 구현 (기본)
// -----------------------------------------------------------------------------

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

// IsRunning은 프로세스가 실행 중인지 확인합니다 (ps + grep).
func (c *ControllerType) IsRunning(run model.Run, comp *model.Component) (bool, error) {
    host := comp.GetHost()
    output, _ := host.ExecLogged("ps ax | grep ziti-controller | grep -v grep")
    return len(output) > 0, nil
}

// Stop은 실행 중인 프로세스를 종료합니다.
func (c *ControllerType) Stop(run model.Run, comp *model.Component) error {
    host := comp.GetHost()
    return host.KillProcesses("-15", func(line string) bool {
        return strings.Contains(line, "ziti-controller")
    })
}

// -----------------------------------------------------------------------------
// FileStagingComponent 인터페이스 구현 (Build 단계)
// -----------------------------------------------------------------------------

func (c *ControllerType) StageFiles(run model.Run, comp *model.Component) error {
    kitBuild := model.KitBuild() // 로컬 스테이징 경로
    
    // 1. Controller 바이너리 복사
    binDest := filepath.Join(kitBuild, "bin", "ziti-controller")
    if err := copyFile(c.BinaryPath, binDest); err != nil {
        return fmt.Errorf("failed to copy binary: %w", err)
    }
    
    // 2. 설정 파일 생성
    configDest := filepath.Join(kitBuild, "cfg", "controller.yml")
    config := generateConfig(comp, run.GetLabel())
    if err := writeFile(configDest, config); err != nil {
        return fmt.Errorf("failed to write config: %w", err)
    }
    
    // 3. 시작 스크립트 생성
    scriptDest := filepath.Join(kitBuild, "scripts", "start-controller.sh")
    script := `#!/bin/bash
/home/ubuntu/fablab/bin/ziti-controller run /home/ubuntu/fablab/cfg/controller.yml
`
    if err := writeFile(scriptDest, []byte(script)); err != nil {
        return fmt.Errorf("failed to write script: %w", err)
    }
    
    // 4. PKI 생성
    pkiDest := filepath.Join(kitBuild, "pki")
    if err := generatePKI(comp, pkiDest); err != nil {
        return fmt.Errorf("failed to generate PKI: %w", err)
    }
    
    return nil
}

// -----------------------------------------------------------------------------
// HostInitializingComponent 인터페이스 구현 (Sync 단계)
// -----------------------------------------------------------------------------

func (c *ControllerType) InitializeHost(run model.Run, comp *model.Component) error {
    host := comp.GetHost()
    
    // 실행 권한 부여 및 로그/데이터 디렉토리 생성
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

// -----------------------------------------------------------------------------
// ServerComponent 인터페이스 구현 (Activate 단계)
// -----------------------------------------------------------------------------

func (c *ControllerType) Start(run model.Run, comp *model.Component) error {
    host := comp.GetHost()
    
    // 1. 기존 프로세스 종료
    host.KillProcesses("-9", func(line string) bool {
        return strings.Contains(line, "ziti-controller")
    })
    
    // 2. 백그라운드 실행 (nohup)
    startCmd := "nohup /home/ubuntu/fablab/scripts/start-controller.sh " +
                "> /home/ubuntu/fablab/logs/controller.log 2>&1 &"
    
    if _, err := host.ExecLogged(startCmd); err != nil {
        return fmt.Errorf("failed to start controller: %w", err)
    }
    
    // 3. 시작 확인 (Health Check)
    time.Sleep(3 * time.Second)
    
    running, err := c.IsRunning(run, comp)
    if err != nil || !running {
        return fmt.Errorf("controller failed to start")
    }
    
    logrus.Infof("✓ Controller started on %s", host.PublicIp)
    return nil
}

// --- Helper Functions (Stubs for brevity) ---
func copyFile(src, dst string) error { return nil }
func writeFile(path string, data []byte) error { return nil }
func generateConfig(c *model.Component, l *model.Label) []byte { return []byte("...") }
func generatePKI(c *model.Component, dst string) error { return nil }
```

---

## 2. Ziti Router 구현 (`zitilib/router.go`)

Router는 Controller에 대한 **의존성**을 가지며, **InitializingComponent** 인터페이스를 사용하여 Controller에 자신을 등록(Enrollment)하는 추가 과정을 수행합니다.

```go
package zitilib

import (
    "fmt"
    "path/filepath"
    "strings"
    "time"
    
    "github.com/openziti/fablab/kernel/model"
    "github.com/sirupsen/logrus"
)

type RouterType struct {
    Version        string
    BinaryPath     string
    Mode           string       // "edge" or "fabric"
    ControllerHost *model.Host  // 의존성: Controller 호스트 참조
}

// -----------------------------------------------------------------------------
// FileStagingComponent 구현 (Build 단계)
// -----------------------------------------------------------------------------

func (r *RouterType) StageFiles(run model.Run, comp *model.Component) error {
    kitBuild := model.KitBuild()
    
    // 1. 바이너리 복사
    binDest := filepath.Join(kitBuild, "bin", "ziti-router")
    if err := copyFile(r.BinaryPath, binDest); err != nil {
        return fmt.Errorf("failed to copy router binary: %w", err)
    }
    
    // 2. 설정 파일 생성 (Controller 주소 포함)
    ctrlAddr := fmt.Sprintf("tls://%s:6262", r.ControllerHost.PublicIp)
    configDest := filepath.Join(kitBuild, "cfg", fmt.Sprintf("router-%s.yml", comp.Id))
    
    config := fmt.Sprintf(`v: 3
ctrl:
  endpoint: %s
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
`, ctrlAddr, comp.Host.PublicIp, comp.Host.PublicIp)
    
    if err := writeFile(configDest, []byte(config)); err != nil {
        return fmt.Errorf("failed to write router config: %w", err)
    }
    
    // 3. 시작 스크립트 생성
    scriptDest := filepath.Join(kitBuild, "scripts", fmt.Sprintf("start-router-%s.sh", comp.Id))
    script := fmt.Sprintf(`#!/bin/bash
/home/ubuntu/fablab/bin/ziti-router run /home/ubuntu/fablab/cfg/router-%s.yml
`, comp.Id)
    
    if err := writeFile(scriptDest, []byte(script)); err != nil {
        return fmt.Errorf("failed to write start script: %w", err)
    }
    
    return nil
}

// -----------------------------------------------------------------------------
// HostInitializingComponent 구현 (Sync 단계)
// -----------------------------------------------------------------------------

func (r *RouterType) InitializeHost(run model.Run, comp *model.Component) error {
    host := comp.GetHost()
    
    cmds := []string{
        "chmod +x /home/ubuntu/fablab/bin/ziti-router",
        fmt.Sprintf("chmod +x /home/ubuntu/fablab/scripts/start-router-%s.sh", comp.Id),
        "mkdir -p /home/ubuntu/fablab/data/router",
        "mkdir -p /home/ubuntu/fablab/logs",
        // Router 전용 커널 튜닝
        "sudo sysctl -w net.ipv4.ip_forward=1",
    }
    
    for _, cmd := range cmds {
        if _, err := host.ExecLogged(cmd); err != nil {
            return fmt.Errorf("failed to exec %s: %w", cmd, err)
        }
    }
    return nil
}

// -----------------------------------------------------------------------------
// InitializingComponent 구현 (Activate-Init 단계)
// -----------------------------------------------------------------------------
// Router를 Controller에 등록(Enroll)하는 핵심 로직입니다.

func (r *RouterType) Init(run model.Run, comp *model.Component) error {
    host := comp.GetHost()
    logrus.Infof("Enrolling router %s to controller...", comp.Id)
    
    // 1. Controller에서 Router 생성 (SSH 실행)
    // - JWT 파일 생성
    createRouterCmd := fmt.Sprintf(`
/home/ubuntu/fablab/bin/ziti edge create edge-router %s \
  -a "public" \
  -o /home/ubuntu/fablab/data/%s.jwt \
  --jwt-output-file /home/ubuntu/fablab/data/%s.jwt
`, comp.Id, comp.Id, comp.Id)
    
    if _, err := r.ControllerHost.ExecLogged(createRouterCmd); err != nil {
        return fmt.Errorf("failed to create router in controller: %w", err)
    }
    
    // 2. JWT 파일을 Router 호스트로 전송 (구현 생략: Controller -> Local -> Router)
    // 실제로는 host.SendData() 등을 사용하여 파일을 복사해야 함
    jwtPath := fmt.Sprintf("/home/ubuntu/fablab/data/%s.jwt", comp.Id)
    
    // 3. Router Enrollment 실행
    enrollCmd := fmt.Sprintf(`
/home/ubuntu/fablab/bin/ziti-router enroll \
  /home/ubuntu/fablab/cfg/router-%s.yml \
  --jwt %s
`, comp.Id, jwtPath)
    
    if _, err := host.ExecLogged(enrollCmd); err != nil {
        return fmt.Errorf("failed to enroll router: %w", err)
    }
    
    return nil
}

// -----------------------------------------------------------------------------
// ServerComponent 구현 (Activate-Start 단계)
// -----------------------------------------------------------------------------

func (r *RouterType) Start(run model.Run, comp *model.Component) error {
    host := comp.GetHost()
    
    // 1. 기존 프로세스 종료
    host.KillProcesses("-9", func(line string) bool {
        return strings.Contains(line, "ziti-router")
    })
    
    // 2. 백그라운드 실행
    startCmd := fmt.Sprintf(
        "nohup /home/ubuntu/fablab/scripts/start-router-%s.sh "+
        "> /home/ubuntu/fablab/logs/router-%s.log 2>&1 &",
        comp.Id, comp.Id,
    )
    
    if _, err := host.ExecLogged(startCmd); err != nil {
        return fmt.Errorf("failed to start router: %w", err)
    }
    
    // 3. 시작 대기 및 연결 확인
    time.Sleep(5 * time.Second)
    
    if err := r.verifyControllerConnection(run, comp); err != nil {
        return fmt.Errorf("router started but failed to connect: %w", err)
    }
    
    logrus.Infof("✓ Router %s started and connected", comp.Id)
    return nil
}

// -----------------------------------------------------------------------------
// Helper Methods
// -----------------------------------------------------------------------------

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
    if strings.Contains(output, `"online":true`) {
        return nil
    }
    return fmt.Errorf("router is not online")
}

// IsRunning, Stop, Label 등 기본 구현은 ControllerType과 유사하므로 생략
func (r *RouterType) Label() string { return "ziti-router" }
func (r *RouterType) IsRunning(run model.Run, comp *model.Component) (bool, error) { return true, nil /* 구현 필요 */ }
func (r *RouterType) Stop(run model.Run, comp *model.Component) error { return nil /* 구현 필요 */ }
```
