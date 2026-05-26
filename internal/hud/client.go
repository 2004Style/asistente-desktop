package hud

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"net"
	"time"
)

type Client struct {
	socketPath string
	eventsChan chan UIEvent
}

func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
		eventsChan: make(chan UIEvent, 200),
	}
}

func (c *Client) Events() <-chan UIEvent {
	return c.eventsChan
}

func (c *Client) Start(ctx context.Context) {
	go func() {
		backoff := 500 * time.Millisecond
		maxBackoff := 10 * time.Second

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Emitir estado disconnected al inicio de la conexión o tras caerse
			c.eventsChan <- UIEvent{
				Type:      "hud.set_state",
				Timestamp: time.Now(),
				Payload:   map[string]interface{}{"state": string(HUDDisconnected)},
			}

			log.Printf("[HUD Client] Conectando a events socket: %s...", c.socketPath)
			conn, err := net.Dial("unix", c.socketPath)
			if err != nil {
				log.Printf("[HUD Client] Error al conectar: %v. Reintentando en %v...", err, backoff)
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
					continue
				}
			}

			// Resetear backoff tras conectar
			backoff = 500 * time.Millisecond
			log.Println("[HUD Client] Conectado exitosamente al socket de eventos.")

			reader := bufio.NewReader(conn)
			
			// Terminar conexión si el contexto expira
			go func() {
				<-ctx.Done()
				_ = conn.Close()
			}()

			for {
				line, err := reader.ReadBytes('\n')
				if err != nil {
					log.Printf("[HUD Client] Conexión perdida: %v", err)
					_ = conn.Close()
					break
				}

				var ev UIEvent
				if err := json.Unmarshal(line, &ev); err != nil {
					log.Printf("[HUD Client] Error decodificando evento: %v", err)
					continue
				}

				select {
				case c.eventsChan <- ev:
				default:
				}
			}
		}
	}()
}
