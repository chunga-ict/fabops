<!-- e4d1a489-bee7-44ec-ac98-a97064e572e2 67647c4f-a435-4878-a380-4d22c64b5f3d -->
# 설정 파일 기반 배포 방식 상세 설명

## 1. 전체 아키텍처 개요

### 이중 모드 지원 구조

```
┌─────────────────────────────────────────────────────────┐
│                    Fablab CLI                           │
│  (cmd/fablab/main.go)                                   │
│                                                         │
│  ┌──────────────────┐      ┌──────────────────┐      │
│  │  코드 기반 모드    │      │  설정 파일 모드    │      │
│  │  (기존 방식)       │      │  (신규 방식)       │      │
│  └────────┬─────────┘      └────────┬─────────┘      │
│           │                         │                 │
│           │                         │                 │
│  ┌────────▼─────────────────────────▼─────────┐      │
│  │      공통 Fablab Kernel                    │      │
│  │  (kernel/model/*)                          │      │
│  │  - Model 구조                              │      │
│  │  - 라이프사이클 관리                        │      │
│  │  - Host/Component 관리                     │      │
│  └───────────────────────────────────────────┘      │
└─────────────────────────────────────────────────────────┘
```

## 2. 모드별 동작 방식

### 모드 1: 코드 기반 모드 (기존 방식 - 존속)

**사용 시나리오**: 개발/테스트 환경, 빠른 프로토타이핑

**구조**:

```
사용자 코드 (main.go)
  └─> InitModel(&model.Model{...})  // Go 코드로 모델 정의
      └─> fablab.Run()              // CLI 실행
          └─> subcmd.Execute()      // 명령어 처리
              └─> model.Bootstrap() // 모델 초기화
                  └─> 라이프사이클 실행
```

**예시**:

```go
// my-model/main.go
package main

import (
    "github.com/openziti/fablab"
    "github.com/openziti/fablab/kernel/model"
)

func main() {
    m := &model.Model{
        Id: "my-ziti-network",
        Regions: model.Regions{
            "us-east-1": &model.Region{
                Id: "us-east-1",
                Hosts: model.Hosts{
                    "controller": &model.Host{
                        Id: "controller",
                        Components: model.Components{
                            "ziti-controller": &model.Component{
                                Id: "ziti-controller",
                                Type: &MyControllerType{},
                            },
                        },
                    },
                },
            },
        },
    }
    
    fablab.InitModel(m)
    fablab.Run()
}
```

**CLI 사용**:

```bash
# 빌드
go build -o my-model ./my-model

# 인스턴스 생성
./my-model create --name test-env

# 배포
./my-model up
```

### 모드 2: 설정 파일 기반 모드 (신규 방식)

**사용 시나리오**: 프로덕션 배포, Kubernetes 환경, 상시 서비스

**구조**:

```
YAML 설정 파일
  └─> ModelLoader.Load()           // YAML 파싱
      └─> Model 빌드                 // Go Model 구조체 생성
          └─> Kubernetes Operator   // 또는 CLI 직접 실행
              └─> model.Bootstrap() // 동일한 초기화
                  └─> 라이프사이클 실행
```

**YAML 예시**:

```yaml
# model.yaml
apiVersion: fablab.openziti.io/v1alpha1
kind: FablabModel
metadata:
  name: ziti-prod-network
spec:
  id: ziti-prod-network
  regions:
    - id: us-east-1
      hosts:
        - id: controller
          instanceType: t3.medium
          components:
            - id: ziti-controller
              type: ziti-controller
              image: openziti/controller:latest
```

**Kubernetes 배포**:

```bash
# Kubernetes에 배포
kubectl apply -f model.yaml

# Operator가 자동으로 처리
# - YAML 로드
# - 모델 생성
# - 라이프사이클 실행
```

**CLI 직접 사용** (개발/테스트):

```bash
# 설정 파일로 직접 실행
fablab run --config model.yaml

# 또는 Operator standalone 모드
fablab-operator --config model.yaml
```

## 3. CLI 구조 존속 방식

### 3.1 기존 CLI 명령어 유지

**모든 기존 명령어가 두 모드에서 동작**:

```bash
# 코드 기반 모드
./my-model create --name test
./my-model up
./my-model start controller
./my-model ssh controller
./my-model status

# 설정 파일 기반 모드
fablab create --config model.yaml --name test
fablab up --config model.yaml
fablab start controller --config model.yaml
fablab ssh controller --config model.yaml
fablab status --config model.yaml
```

### 3.2 CLI 진입점 확장

**cmd/fablab/main.go 수정**:

```go
func main() {
    // --config 플래그 확인
    configPath := flag.String("config", "", "Path to YAML config file")
    flag.Parse()
    
    if *configPath != "" {
        // 설정 파일 모드
        runConfigMode(*configPath)
    } else {
        // 기존 코드 기반 모드
        runCodeMode()
    }
}

func runConfigMode(configPath string) {
    // YAML 로더 사용
    loader := loader.NewModelLoader(configPath)
    m, err := loader.Load()
    if err != nil {
        logrus.Fatal(err)
    }
    
    // 모델 초기화
    model.InitModel(m)
    
    // 기존 CLI 실행 (동일한 subcmd 사용)
    if err := subcmd.Execute(); err != nil {
        logrus.Fatal(err)
    }
}

func runCodeMode() {
    // 기존 로직 유지
    cfg := model.GetConfig()
    instance, ok := cfg.Instances[cfg.Default]
    // ... 기존 코드
}
```

### 3.3 인스턴스 관리 통합

**기존 인스턴스 시스템과 통합**:

```
~/.fablab/
├── config.yaml              # 전역 설정
├── instances/
│   ├── test-env/            # 코드 기반 인스턴스
│   │   ├── label.json
│   │   └── working/
│   └── prod-network/        # 설정 파일 기반 인스턴스
│       ├── label.json
│       ├── model.yaml       # 사용된 설정 파일 복사
│       └── working/
```

**인스턴스 생성 통합**:

```go
// cmd/fablab/subcmd/create.go 확장
func (self *CreateCommand) create(cmd *cobra.Command, args []string) error {
    var m *model.Model
    
    if self.ConfigPath != "" {
        // 설정 파일 모드
        loader := loader.NewModelLoader(self.ConfigPath)
        var err error
        m, err = loader.Load()
        if err != nil {
            return err
        }
    } else {
        // 코드 기반 모드 (기존)
        m = model.GetModel()
        if m == nil {
            return errors.New("no model configured")
        }
    }
    
    // 공통 인스턴스 생성 로직
    instanceId, err := model.NewInstance(self.Name, self.WorkingDir, self.Executable)
    // ... 나머지 동일
}
```

## 4. Kubernetes Operator 통합

### 4.1 Operator와 CLI 공존

**Operator는 내부적으로 동일한 Fablab Kernel 사용**:

```go
// kernel/operator/controller.go
func (r *FablabModelReconciler) reconcileModel(...) {
    // YAML 로드
    loader := loader.NewModelLoader(configPath)
    m, err := loader.Load()
    
    // 모델 초기화 (전역 변수 대신 인스턴스 저장소 사용)
    modelStore.Set(fablabModel.Name, m)
    
    // Bootstrap (동일한 로직)
    if err := model.Bootstrap(); err != nil {
        return err
    }
    
    // 라이프사이클 실행 (동일한 메서드)
    run, _ := model.NewRun()
    m.Express(run)
    m.Build(run)
    // ...
}
```

### 4.2 CLI에서 Operator 리소스 관리

**CLI로 Kubernetes 리소스 조회/관리**:

```bash
# Operator로 배포된 모델 조회
fablab list --k8s

# Kubernetes 리소스 상태 확인
fablab status --k8s --instance prod-network

# CLI로 직접 실행 (Operator 우회)
fablab up --config model.yaml --local
```

## 5. 데이터 흐름

### 5.1 설정 파일 → 모델 변환

```
YAML 파일
  │
  ├─> ModelLoader.Load()
  │   ├─> YAML 파싱
  │   ├─> ModelSpec 구조체 생성
  │   └─> buildModel()
  │       ├─> Region 생성
  │       ├─> Host 생성
  │       ├─> Component 생성
  │       └─> Stage/Action 설정
  │
  └─> model.Model (Go 구조체)
      └─> 기존 코드와 동일한 구조
```

### 5.2 모델 실행 흐름

```
모델 (코드 또는 YAML)
  │
  ├─> model.InitModel()
  │   └─> 전역 변수 설정 (코드 모드)
  │   └─> 인스턴스 저장소 (설정 파일 모드)
  │
  ├─> model.Bootstrap()
  │   ├─> Factory 실행
  │   ├─> 변수 바인딩
  │   └─> Label 로드
  │
  └─> 라이프사이클 실행
      ├─> Express()  (Infrastructure)
      ├─> Build()    (Configuration)
      ├─> Sync()     (Distribution)
      ├─> Activate() (Activation)
      └─> Operate()  (Operation)
```

## 6. 호환성 보장

### 6.1 기존 코드 호환성

**기존 모델 코드는 그대로 동작**:

```go
// 기존 코드 (변경 불필요)
func main() {
    m := &model.Model{...}
    fablab.InitModel(m)
    fablab.Run()
}
```

### 6.2 설정 파일 → 코드 변환

**YAML을 Go 코드로 변환하는 도구 제공** (선택사항):

```bash
# YAML을 Go 코드로 변환
fablab convert --config model.yaml --output model.go
```

### 6.3 하이브리드 모드

**코드와 설정 파일 혼합 사용**:

```go
func main() {
    // 기본 구조는 코드로
    m := &model.Model{
        Id: "my-network",
        // ...
    }
    
    // 일부 설정은 YAML에서 오버라이드
    if configPath := os.Getenv("FABLAB_CONFIG"); configPath != "" {
        loader := loader.NewModelLoader(configPath)
        yamlModel, _ := loader.Load()
        // YAML 설정으로 병합
        mergeModel(m, yamlModel)
    }
    
    fablab.InitModel(m)
    fablab.Run()
}
```

## 7. 구현 파일 구조

### 7.1 신규 파일

```
kernel/
├── model/
│   └── loader/
│       ├── yaml.go          # YAML 로더 (이미 존재)
│       └── builder.go       # Model 빌더 (확장 필요)
│
cmd/
├── fablab/
│   └── main.go              # CLI 진입점 (수정)
└── fablab-operator/
    └── main.go              # Operator 진입점 (이미 존재)

kernel/
└── operator/
    ├── controller.go        # Kubernetes Controller (이미 존재)
    └── manager.go          # Manager 설정 (이미 존재)
```

### 7.2 수정 파일

```
cmd/fablab/
├── main.go                  # 모드 선택 로직 추가
└── subcmd/
    ├── create.go           # --config 플래그 지원
    ├── up.go              # 설정 파일 모드 지원
    └── root.go            # 전역 --config 플래그

kernel/model/
├── globals.go             # 인스턴스 저장소 추가 (선택)
└── instance.go           # 설정 파일 경로 저장
```

## 8. 사용 예시

### 8.1 개발 환경 (코드 기반)

```bash
# 1. 모델 코드 작성
cd my-ziti-model
vim main.go  # Go 코드로 모델 정의

# 2. 빌드
go build -o ziti-model

# 3. 사용
./ziti-model create --name dev
./ziti-model up
```

### 8.2 프로덕션 환경 (설정 파일 기반)

```bash
# 1. YAML 작성
vim production-model.yaml

# 2. Kubernetes 배포
kubectl apply -f production-model.yaml

# 3. CLI로 상태 확인
fablab status --k8s --instance prod-network
```

### 8.3 하이브리드 사용

```bash
# 설정 파일로 빠른 시작
fablab create --config quick-start.yaml --name test

# 코드로 세밀한 제어
./custom-model start router-1
```

## 9. 핵심 설계 원칙

1. **하위 호환성**: 기존 코드는 변경 없이 동작
2. **공통 Kernel**: 두 모드 모두 동일한 Fablab Kernel 사용
3. **점진적 전환**: 코드 → 설정 파일로 점진적 마이그레이션 가능
4. **CLI 통합**: 모든 명령어가 두 모드에서 동작
5. **인스턴스 통합**: 동일한 인스턴스 관리 시스템 사용

## 10. 마이그레이션 경로

### 단계 1: 설정 파일 지원 추가

- YAML 로더 구현
- CLI에 --config 플래그 추가
- 기존 코드는 그대로 동작

### 단계 2: Operator 통합

- Kubernetes Operator 구현
- CLI와 Operator 공존

### 단계 3: 점진적 전환

- 기존 모델을 YAML로 변환 (선택)
- 새 모델은 YAML로 작성
- 코드 모드는 계속 지원

### To-dos

- [ ] 

