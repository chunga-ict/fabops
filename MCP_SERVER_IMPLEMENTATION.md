# Fablab MCP(Model Context Protocol) 서버 구현 상세

사용자께서 관심을 보이신 **MCP Server 도입**에 대해, Fablab 아키텍처 관점에서의 심층 분석과 Go 언어 기반의 구체적인 구현 방안을 정리했습니다.

## 1. MCP란 무엇인가?
**Model Context Protocol (MCP)**는 Claude, IDE(Cursor), AI 에이전트 등의 **"AI 모델"**과 로컬/원격 시스템의 **"데이터 및 기능"**을 표준화된 방식으로 연결해주는 프로토콜입니다.

*   기존: 각 AI 서비스마다 독자적인 플러그인/Tool spec을 만들어야 했음.
*   **MCP**: 한 번 표준에 맞춰 서버를 만들면, Claude Desktop, Cursor, 기타 MCP 지원 에이전트들이 즉시 연결하여 사용할 수 있음.

## 2. Fablab에 MCP가 필요한 이유 (Why?)

### A. "ChatOps"의 완성
복잡한 CLI 명령어(`fablab create --model aws ...`)를 외울 필요 없이, AI에게 "AWS에 테스트용 Ziti 네트워크 하나 만들어줘"라고 말하면 AI가 적절한 Tool을 호출합니다.

### B. Digital Twin의 "통역사"
Fablab의 상태 저장소(Digital Twin)에 저장된 복잡한 JSON/Data 구조를 AI가 읽기 편한 텍스트로 변환(Context)하여 제공할 수 있습니다. AI는 이 정보를 바탕으로 인프라 상태를 진단하고 조언할 수 있습니다.

---

## 3. 구현 아키텍처 (Architecture)

Go 언어 기반의 MCP 서버를 Fablab 바이너리에 내장하거나 별도 데몬으로 띄우는 방식입니다.

### 3.1 기술 스택
*   **언어**: Go (Golang)
*   **SDK**: `github.com/mark3labs/mcp-go` (Go 언어용 MCP 표준 라이브러리)
*   **Transport**:
    *   `Stdio`: 로컬 컴퓨터에서 Claude Desktop app 등과 직접 연결할 때 사용 (Default).
    *   `SSE`: 원격 서버에서 실행 시 HTTP(Server-Sent Events) 사용.

### 3.2 핵심 구성 요소

MCP 프로토콜의 3대 요소(Resources, Tools, Prompts)를 Fablab에 매핑합니다.

#### A. Resources (읽기 전용 데이터)
AI에게 현재 Digital Twin의 상태를 보여주는 "파일"과 같은 개념입니다.

| URI Scheme | 설명 | 예시 데이터 |
| :--- | :--- | :--- |
| `fablab://status` | 전체 네트워크의 건강 상태 요약 | `{"status": "Healthy", "controller": "Up", "router_count": 3}` |
| `fablab://topology` | 네트워크 구조 (Region, Host 매핑) | `model.json` 덤프 데이터 |
| `fablab://logs/{host}` | 특정 호스트의 최근 로그 | SSH로 가져온 로그의 마지막 100줄 |

#### B. Tools (실행 가능한 함수)
AI가 직접 호출하여 Fablab 상태를 변경할 수 있는 도구(함수)들입니다.

| Tool Name | Arguments | 로직 |
| :--- | :--- | :--- |
| `create_network` | `{"name": string, "provider": string}` | `fablab create` 로직 실행 |
| `scale_router` | `{"count": int, "region": string}` | 인스턴스 설정 수정 후 Reconciliation 트리거 |
| `run_diagnostics` | `{"compliance_check": bool}` | 네트워크 연결 테스트 수행 결과 반환 |

#### C. Prompts (미리 정의된 질문 템플릿)
사용자가 자주 묻는 질문을 템플릿화하여 제공합니다.

*   **"Analyze Network Health"**: `fablab://status`와 `fablab://logs` 리소스를 한꺼번에 AI 컨텍스트에 넣고 "지금 문제가 뭔지 분석해줘"라고 요청하는 프롬프트.

---

## 4. 구현 코드 예시 (Go)

```go
package mcp_server

import (
	"context"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/openziti/fablab/kernel/model"
)

// MCP 서버 인스턴스 생성
func NewFablabMCPServer() *server.MCPServer {
	s := server.NewMCPServer(
		"Fablab Digital Twin",
		"v1.0.0",
	)

	// 1. Tool 등록: 네트워크 생성
	s.AddTool(mcp.NewTool("create_network",
		mcp.WithDescription("Create a new OpenZiti network instance"),
		mcp.WithString("name", "Name of the network instance", true),
		mcp.WithString("provider", "Infrastructure provider (aws, local)", true),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := request.Arguments["name"].(string)
		// Fablab Kernel 호출
		result, err := model.CreateNetwork(name) 
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Network '%s' created successfully!", name)), nil
	})

	// 2. Resource 등록: 상태 조회
	s.AddResource(mcp.NewResource("fablab://status",
		mcp.WithDescription("Current status of the Fablab network"),
		mcp.WithMimeType("application/json"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ReadResourceResult, error) {
		// Digital Twin 상태 저장소 읽기
		statusJson := model.GetStatusJson()
		return []mcp.ReadResourceResult{
			{
				Uri: "fablab://status",
				MimeType: "application/json",
				Text: statusJson,
			},
		}, nil
	})

	return s
}
```

## 5. 단계별 로드맵 제안

1.  **Phase 1: Read-Only Agent (모니터링)**
    *   `Resources`만 먼저 구현합니다. AI가 현재 인프라 상태를 읽고 "로그 분석"이나 "상태 진단"만 수행하게 합니다. 가장 안전한 시작입니다.
2.  **Phase 2: Active Agent (운영 자동화)**
    *   `Tools`를 열어줍니다. 처음에는 비파괴적인 작업(예: `restart_service`)부터 시작하여, 점차 인프라 생성/삭제 권한을 부여합니다.
3.  **Phase 3: Autonomous Operator (자율 운영)**
    *   AI가 주기적으로 `fablab://status`를 확인하고, 문제가 감지되면 스스로 `scale_router` Tool을 호출하여 복구하는 완전 자동화 단계입니다.

## 6. 결론
MCP 서버를 구현하면 Fablab은 단순한 "명령줄 도구"를 넘어 **"AI가 이해하고 다룰 수 있는 스마트 인프라 플랫폼"**이 됩니다. 이는 사용자(개발자)가 코드를 짜는 시간보다 AI와 협업하여 인프라 레벨을 설계하는 데 더 집중할 수 있게 해줍니다.
