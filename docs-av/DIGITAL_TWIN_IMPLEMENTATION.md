# Fablab의 "Digital Twin" 구현 분석

Fablab은 **Structural Model(구조적 모델)**과 **Feedback Loop(피드백 루프)**를 통해 물리적/클라우드 인프라의 상태를 소프트웨어적으로 복제하는 "Digital Twin" 역할을 수행합니다. 이 문서는 그 상세 구현 메커니즘을 분석한 내용입니다.

## 1. Digital Twin의 3요소 매핑

| Digital Twin 개념 | Fablab 코드 매핑 | 역할 |
| :--- | :--- | :--- |
| **Ideal Model (목표 상태)** | `kernel/model/model.go` (Model 구조체) | 사용자가 정의한 인프라의 청사진. Region, Host, Component의 계층 구조를 가집니다. |
| **Real State (현실 상태)** | `kernel/model/label.go` (Label 구조체) | 실제 배포된 인프라에서 수집된 동적 데이터(IP, ID 등)를 키-값 쌍(`Bindings`)으로 저장하는 영속적 저장소입니다. |
| **Sync Mechanism (동기화)** | `terraform.go` (bind 함수) | 실제 인프라의 출력값(Output)을 읽어와 메모리 상의 모델에 주입하는 과정입니다. |

## 2. 상세 구현 메커니즘

### 2.1 모델 정의 (The Blueprint)
`kernel/model/model.go` 파일의 `Model` 구조체는 전체 시스템의 논리적 구성을 정의합니다. 이 시점에서는 구체적인 IP 주소나 클라우드 리소스 ID 등을 알 수 없습니다.

```go
type Model struct {
    Regions Regions
    // ...
}
type Host struct {
    PublicIp string // 초기에는 비어 있음 ("")
    // ...
}
```

### 2.2 인프라 생성 및 상태 수집 (The Actuation & Sensing)
`kernel/lib/runlevel/0_infrastructure/terraform/terraform.go` 파일에서 실제 인프라 생성(`terraform apply`)과 상태 수집이 발생합니다.

*   `Express()` 단계에서 Terraform을 실행하여 AWS EC2 등의 리소스를 생성합니다.
*   **핵심 로직**: `bind()` 메서드가 Terraform의 출력값(Output)을 파싱하여 `l.Bindings`에 저장합니다.

```go
// terraform.go (요약)
func (t *Terraform) bind(m *model.Model, l *model.Label) error {
    // 1. Terraform Output 조회 (Sensing)
    output := allTerraformOutput() 
    
    // 2. Output을 Label에 매핑
    for regionId, region := range m.Regions {
        for hostId := range region.Hosts {
            // 예: "us-east-1_host_controller_public_ip" 키 생성
            key := fmt.Sprintf("%s_host_%s_public_ip", regionId, hostId)
            l.Bindings[key] = output[key] // 수집된 실제 IP 저장
        }
    }
    
    // 3. 상태 영속화 (Snapshot)
    l.Save()
    
    // 4. 메모리 모델 동기화 (Feedback)
    m.BindLabel(l)
}
```

### 2.3 모델 동기화 (The Synchronization)
`kernel/model/label.go`의 `BindLabel` 메서드는 수집된 데이터를 메모리 상의 구조체(`Host`)에 주입하여, **추상 모델을 구체적인 Digital Twin으로 변환**합니다.

```go
// label.go
func (m *Model) BindLabel(l *Label) {
    for _, region := range m.Regions {
        for _, host := range region.Hosts {
            // Label(실제 상태)에서 값을 읽어 Host(모델) 필드 채움
            if ip, ok := l.Bindings[bindingKey]; ok {
                host.PublicIp = ip 
            }
        }
    }
}
```

## 3. Digital Twin 활용

이렇게 동기화된 Digital Twin(채워진 Model 객체)은 다음과 같이 활용됩니다.

1.  **원격 제어 (Remote Control)**: `Host.PublicIp`가 채워졌으므로, Fablab은 SSH를 통해 해당 호스트에 접속하여 명령을 내릴 수 있습니다. (예: `fablab ssh`)
2.  **구성 관리 (Configuration)**: 소프트웨어 설정 파일 생성 시, 실제 할당된 IP 주소를 템플릿에 주입하여 정확한 설정 파일을 만듭니다.
3.  **가시성 (Observability)**: `fablab dump` 명령어를 통해 현재 전체 인프라의 상태를 JSON으로 시각화할 수 있습니다.

## 4. 요약
Fablab의 Digital Twin 구현은 **"정적 모델 정의(Go Struct) → 인프라 프로비저닝(Terraform) → 동적 상태 수집(Output Parsing) → 런타임 모델 바인딩(BindLabel)"**의 4단계 루프를 통해 완성됩니다.
