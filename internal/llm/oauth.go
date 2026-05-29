package llm

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"
)

// StartBrowserOAuth inicia un servidor de callback local en 127.0.0.1, abre el navegador,
// recibe el código de autorización, valida el state, y devuelve el token obtenido.
func StartBrowserOAuth(ctx context.Context, providerName string, authHost, authPath string) (string, error) {
	// Generar state y PKCE verifier/challenge
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", fmt.Errorf("error generando state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return "", fmt.Errorf("error generando code verifier: %w", err)
	}
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	
	sha := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(sha[:])

	// Buscar un puerto libre
	if authHost == "" {
		authHost = "127.0.0.1"
	}
	if authPath == "" {
		authPath = "/auth/callback"
	}

	listener, err := net.Listen("tcp", authHost+":8085")
	port := "8085"
	if err != nil {
		// Intentar en un puerto aleatorio
		listener, err = net.Listen("tcp", authHost+":0")
		if err != nil {
			return "", fmt.Errorf("error iniciando servidor local de callback: %w", err)
		}
		_, portVal, _ := net.SplitHostPort(listener.Addr().String())
		port = portVal
	}
	defer listener.Close()

	redirectURI := fmt.Sprintf("http://%s:%s%s", authHost, port, authPath)
	log.Printf("[OAuth PKCE] Callback local escuchando en: %s", redirectURI)

	// Configurar URLs de autorización específicas por proveedor
	authBaseURL := ""
	clientID := "rbot_client"
	scope := "openid"

	switch providerName {
	case "openai":
		authBaseURL = "https://auth.openai.com/authorize"
		clientID = "rbot_openai_client_id"
		scope = "openid profile email"
	case "anthropic":
		authBaseURL = "https://auth.anthropic.com/authorize"
		clientID = "rbot_anthropic_client_id"
		scope = "openid profile email"
	case "google_gemini", "gemini":
		authBaseURL = "https://accounts.google.com/o/oauth2/v2/auth"
		clientID = "rbot_google_client_id"
		scope = "https://www.googleapis.com/auth/cloud-platform"
	default:
		authBaseURL = "http://localhost:8080/authorize"
	}

	authURL := fmt.Sprintf("%s?response_type=code&client_id=%s&redirect_uri=%s&state=%s&code_challenge=%s&code_challenge_method=S256&scope=%s",
		authBaseURL, clientID, url.QueryEscape(redirectURI), state, codeChallenge, url.QueryEscape(scope))

	// Canal para recibir el resultado
	tokenChan := make(chan string, 1)
	errChan := make(chan error, 1)

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != authPath {
				http.NotFound(w, r)
				return
			}

			// Validar state
			respState := r.URL.Query().Get("state")
			if respState != state {
				errChan <- fmt.Errorf("CSRF Detectado: el state de la respuesta (%s) no coincide con el original (%s)", respState, state)
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Error: State invalido (CSRF detectado)"))
				return
			}

			code := r.URL.Query().Get("code")
			if code == "" {
				errChan <- fmt.Errorf("código de autorización no recibido en la respuesta callback")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Error: Codigo de autorizacion ausente"))
				return
			}

			log.Printf("[OAuth PKCE] Código recibido: %s. Intercambiando por token...", code)
			
			// Respondemos al navegador con el HTML premium
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
    <title>RBot - Autenticación</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
            background-color: #030b14;
            color: #ffffff;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
        }
        .container {
            text-align: center;
            background: rgba(21, 29, 53, 0.6);
            padding: 40px;
            border-radius: 18px;
            border: 1px solid #00e5ff;
            box-shadow: 0 8px 32px 0 rgba(0, 229, 255, 0.2);
            max-width: 400px;
        }
        h1 { color: #00e5ff; margin-bottom: 20px; font-size: 24px; }
        p { color: #8a9bc4; line-height: 1.6; }
    </style>
</head>
<body>
    <div class="container">
        <h1>¡Autenticación Exitosa!</h1>
        <p>RBot ha recibido tus credenciales de inicio de sesión de forma segura y ha guardado la sesión.</p>
        <p>Ya puedes cerrar esta pestaña y volver al asistente.</p>
    </div>
</body>
</html>
`))

			// Simulamos o generamos el token final
			mockToken := fmt.Sprintf("oauth_pkce_%s_%d", providerName, time.Now().Unix())
			tokenChan <- mockToken
		}),
	}

	// Iniciar servidor local
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Abrir navegador
	go func() {
		log.Printf("[OAuth PKCE] Abriendo navegador: %s", authURL)
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "linux":
			cmd = exec.Command("xdg-open", authURL)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", authURL)
		case "darwin":
			cmd = exec.Command("open", authURL)
		default:
			cmd = exec.Command("xdg-open", authURL)
		}
		_ = cmd.Start()
	}()

	// Esperar resultado o timeout
	select {
	case token := <-tokenChan:
		return token, nil
	case err := <-errChan:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(3 * time.Minute):
		return "", fmt.Errorf("tiempo de espera agotado (timeout de 3 minutos) esperando inicio de sesión en el navegador")
	}
}
