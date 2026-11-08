package cep

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// um client só pros dois providers. O http.Client pode ser usado por várias
// goroutines ao mesmo tempo e ainda reaproveita as conexões.
var defaultClient = &http.Client{Timeout: 10 * time.Second}

// devolve o status junto com o erro porque cada API avisa que não achou o CEP de
// um jeito, então quem decide o que fazer é o provider
func getJSON(ctx context.Context, client *http.Client, url string, dst any) (int, error) {
	// é o NewRequestWithContext que deixa cancelar a requisição depois
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("montando requisição: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("executando requisição: %w", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return resp.StatusCode, fmt.Errorf("decodificando resposta (status %d): %w", resp.StatusCode, err)
	}
	return resp.StatusCode, nil
}
