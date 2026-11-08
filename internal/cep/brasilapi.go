package cep

import (
	"context"
	"fmt"
	"net/http"
)

const BrasilAPIBaseURL = "https://brasilapi.com.br/api/cep/v1"

// BaseURL é campo e não constante pra poder apontar pro servidor falso no teste
type BrasilAPI struct {
	BaseURL string
	Client  *http.Client
}

func NewBrasilAPI() *BrasilAPI {
	return &BrasilAPI{BaseURL: BrasilAPIBaseURL, Client: defaultClient}
}

func (b *BrasilAPI) Name() string { return "BrasilAPI" }

type brasilAPIResponse struct {
	CEP          string `json:"cep"`
	State        string `json:"state"`
	City         string `json:"city"`
	Neighborhood string `json:"neighborhood"`
	Street       string `json:"street"`
}

func (b *BrasilAPI) Fetch(ctx context.Context, cep string) (Address, error) {
	url := fmt.Sprintf("%s/%s", b.BaseURL, cep)

	var body brasilAPIResponse
	status, err := getJSON(ctx, b.Client, url, &body)
	switch {
	// aqui o CEP inexistente vem como 404 mesmo. Checo antes do err pra um
	// problema de parse não esconder isso.
	case status == http.StatusNotFound:
		return Address{}, ErrCEPNaoEncontrado
	case err != nil:
		return Address{}, err
	case status != http.StatusOK:
		return Address{}, fmt.Errorf("status inesperado: %d", status)
	}

	return Address{
		CEP:        Format(body.CEP),
		Logradouro: body.Street,
		Bairro:     body.Neighborhood,
		Cidade:     body.City,
		UF:         body.State,
	}, nil
}
