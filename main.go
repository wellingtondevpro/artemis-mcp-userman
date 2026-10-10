package main

import (
	"context"
	"fmt"
	"os"
	"flag"
	"net/http"
	"net/url"
	"io"
	"bytes"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	transport := flag.String("transport", "sse", "Transport to use: stdio or sse")
	port := flag.Int("port", 8080, "Port for SSE transport")
	flag.Parse()

	mcpServer := server.NewMCPServer(
		"artemis-mcp-userman",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
	)

	// Registra tools do UserMan
	registerTools(mcpServer)

	if *transport == "sse" {
		addr := fmt.Sprintf(":%d", *port)
		fmt.Printf("Iniciando MCP Server de UserMan em Go (via SSE) na porta %d...\n", *port)
		sseServer := server.NewSSEServer(mcpServer)
		if err := sseServer.Start(addr); err != nil {
			fmt.Fprintf(os.Stderr, "Erro no MCP Server (SSE): %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("Iniciando MCP Server de UserMan em Go (via STDIO)...")
		if err := server.ServeStdio(mcpServer); err != nil {
			fmt.Fprintf(os.Stderr, "Erro no MCP Server (STDIO): %v\n", err)
			os.Exit(1)
		}
	}
}

func registerTools(s *server.MCPServer) {
	// Tool 1: Buscar perfil de usuário
	buscarUsuarioTool := mcp.NewTool("userman_buscar_perfil",
		mcp.WithDescription("Use esta ferramenta para consultar/verificar os dados de um usuário, descobrir seu e-mail ou checar se o status dele está ATIVO ou inativo."),
		mcp.WithString("username", mcp.Required(), mcp.Description("O username do usuário (ex: wellington)")),
	)

	s.AddTool(buscarUsuarioTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("Argumentos inválidos ou nulos"), nil
		}

		username, ok := args["username"].(string)
		if !ok {
			return mcp.NewToolResultError("Argumento 'username' é obrigatório"), nil
		}

		url := fmt.Sprintf("http://localhost:8084/api/users/%s", username)
		fmt.Printf("[MCP LOG] Buscando usuario na API: %s\n", url)
		resp, err := http.Get(url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Erro ao conectar na API Java: %v", err)), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode == 404 {
			return mcp.NewToolResultText("Usuário não encontrado."), nil
		}
		
		if resp.StatusCode != 200 {
			return mcp.NewToolResultError(fmt.Sprintf("Erro da API: Status %d", resp.StatusCode)), nil
		}

		respData, _ := io.ReadAll(resp.Body)
		return mcp.NewToolResultText(fmt.Sprintf("Dados do usuário:\n%s", string(respData))), nil
	})

	// Tool 2: Desativar usuário
	desativarUsuarioTool := mcp.NewTool("userman_desativar_acesso",
		mcp.WithDescription("AÇÃO DESTRUTIVA. Use APENAS quando o usuário pedir explicitamente para bloquear, deletar ou desativar o acesso de alguém."),
		mcp.WithString("username", mcp.Required(), mcp.Description("O username a ser desativado")),
	)

	s.AddTool(desativarUsuarioTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("Argumentos inválidos ou nulos"), nil
		}

		username, ok := args["username"].(string)
		if !ok {
			return mcp.NewToolResultError("Argumento 'username' é obrigatório"), nil
		}

		url := fmt.Sprintf("http://localhost:8084/api/users/%s/deactivate", username)
		fmt.Printf("[MCP LOG] Desativando usuario na API: %s\n", url)
		
		reqHttp, _ := http.NewRequest(http.MethodPost, url, nil)
		resp, err := http.DefaultClient.Do(reqHttp)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Erro ao conectar na API Java: %v", err)), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode == 404 || resp.StatusCode == 500 { // fallback for 500 in case it throws exception for not found
			return mcp.NewToolResultText("Falha ao desativar. Usuário não encontrado."), nil
		}

		respData, _ := io.ReadAll(resp.Body)
		return mcp.NewToolResultText(fmt.Sprintf("Usuário desativado com sucesso:\n%s", string(respData))), nil
	})

	// Tool 3: Adicionar Perfil
	adicionarPerfilTool := mcp.NewTool("userman_adicionar_perfil",
		mcp.WithDescription("Use esta ferramenta para adicionar ou atribuir um perfil (role), como ADM, a um usuário."),
		mcp.WithString("username", mcp.Required(), mcp.Description("O username do usuário (ex: wellington)")),
		mcp.WithString("perfil", mcp.Required(), mcp.Description("O perfil a ser adicionado (ex: ADM)")),
	)

	s.AddTool(adicionarPerfilTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("Argumentos inválidos ou nulos"), nil
		}

		username, ok := args["username"].(string)
		if !ok {
			return mcp.NewToolResultError("Argumento 'username' é obrigatório"), nil
		}

		perfil, ok := args["perfil"].(string)
		if !ok {
			return mcp.NewToolResultError("Argumento 'perfil' é obrigatório"), nil
		}

		url := fmt.Sprintf("http://localhost:8084/api/users/%s/roles/%s", username, perfil)
		fmt.Printf("[MCP LOG] Adicionando perfil na API: %s\n", url)
		
		reqHttp, _ := http.NewRequest(http.MethodPost, url, nil)
		resp, err := http.DefaultClient.Do(reqHttp)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Erro ao conectar na API Java: %v", err)), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode == 404 || resp.StatusCode == 500 {
			return mcp.NewToolResultText("Falha ao adicionar perfil. Usuário ou perfil não encontrado."), nil
		}

		respData, _ := io.ReadAll(resp.Body)
		return mcp.NewToolResultText(fmt.Sprintf("Perfil adicionado com sucesso:\n%s", string(respData))), nil
	})

	// Tool: Criar usuário
	criarUsuarioTool := mcp.NewTool("userman_criar_usuario",
		mcp.WithDescription("Use esta ferramenta para criar um novo usuário no USERMAN."),
		mcp.WithString("username", mcp.Required(), mcp.Description("O username do usuário (ex: psantos)")),
		mcp.WithString("email", mcp.Required(), mcp.Description("O email do usuário")),
	)

	s.AddTool(criarUsuarioTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("Argumentos inválidos ou nulos"), nil
		}

		username, ok := args["username"].(string)
		if !ok {
			return mcp.NewToolResultError("Argumento 'username' é obrigatório"), nil
		}

		email, ok := args["email"].(string)
		if !ok {
			return mcp.NewToolResultError("Argumento 'email' é obrigatório"), nil
		}

		payload := map[string]interface{}{
			"username": username,
			"email":    email,
		}
		bodyBytes, _ := json.Marshal(payload)

		url := "http://localhost:8084/api/users"
		fmt.Printf("[MCP LOG] Criando usuario na API: %s\n", url)
		
		reqHttp, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(bodyBytes))
		reqHttp.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(reqHttp)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Erro ao conectar na API Java: %v", err)), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode == 409 {
			return mcp.NewToolResultText("Falha ao criar usuário. Username ou email já existe."), nil
		}
		
		if resp.StatusCode != 200 && resp.StatusCode != 201 {
			return mcp.NewToolResultError(fmt.Sprintf("Erro da API ao criar usuario: Status %d", resp.StatusCode)), nil
		}

		respData, _ := io.ReadAll(resp.Body)
		return mcp.NewToolResultText(fmt.Sprintf("Usuário criado com sucesso:\n%s", string(respData))), nil
	})



	// Tool 4: Busca Avançada
	buscaAvancadaTool := mcp.NewTool("userman_buscar_usuarios_avancado",
		mcp.WithDescription("Use para buscas genéricas de usuários combinando parâmetros como role (ADM), active (true/false), email, ou username. Opcionais."),
		mcp.WithString("role", mcp.Description("Nome do perfil/role para buscar (ex: ADM)")),
		mcp.WithString("active", mcp.Description("Se o usuário está ativo (true) ou inativo (false)")),
		mcp.WithString("username", mcp.Description("Busca parcial por nome de usuário")),
	)

	s.AddTool(buscaAvancadaTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := request.Params.Arguments.(map[string]interface{})
		
		reqUrl, _ := url.Parse("http://localhost:8084/api/users")
		q := reqUrl.Query()
		
		if role, ok := args["role"].(string); ok && role != "" {
			q.Add("role", role)
		}
		if active, ok := args["active"].(string); ok && active != "" {
			q.Add("active", active)
		}
		if username, ok := args["username"].(string); ok && username != "" {
			q.Add("username", username)
		}
		
		reqUrl.RawQuery = q.Encode()
		finalUrl := reqUrl.String()
		fmt.Printf("[MCP LOG] Busca avancada na API: %s\n", finalUrl)
		
		resp, err := http.Get(finalUrl)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Erro ao conectar na API Java: %v", err)), nil
		}
		defer resp.Body.Close()

		respData, _ := io.ReadAll(resp.Body)
		return mcp.NewToolResultText(fmt.Sprintf("Resultados:\n%s", string(respData))), nil
	})

	// Tool 5: Atualizar usuário
	atualizarUsuarioTool := mcp.NewTool("userman_atualizar_usuario",
		mcp.WithDescription("Use esta ferramenta para atualizar dados de um usuário (email ou status ativo)."),
		mcp.WithString("username", mcp.Required(), mcp.Description("O username a ser atualizado")),
		mcp.WithString("email", mcp.Description("O novo email do usuário")),
		mcp.WithBoolean("active", mcp.Description("O status ativo do usuário")),
	)

	s.AddTool(atualizarUsuarioTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("Argumentos inválidos ou nulos"), nil
		}

		username, ok := args["username"].(string)
		if !ok {
			return mcp.NewToolResultError("Argumento 'username' é obrigatório"), nil
		}

		payload := make(map[string]interface{})
		if email, ok := args["email"].(string); ok && email != "" {
			payload["email"] = email
		}
		if active, ok := args["active"].(bool); ok {
			payload["active"] = active
		}

		bodyBytes, _ := json.Marshal(payload)
		url := fmt.Sprintf("http://localhost:8084/api/users/%s", username)
		fmt.Printf("[MCP LOG] Atualizando usuario na API: %s\n", url)

		reqHttp, _ := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(bodyBytes))
		reqHttp.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(reqHttp)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Erro ao conectar na API Java: %v", err)), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode == 404 {
			return mcp.NewToolResultText("Falha ao atualizar. Usuário não encontrado."), nil
		}

		respData, _ := io.ReadAll(resp.Body)
		return mcp.NewToolResultText(fmt.Sprintf("Usuário atualizado com sucesso:\n%s", string(respData))), nil
	})
}

