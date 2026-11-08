package cep

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

const ViaCEPBaseURL = "http://viacep.com.br/ws"

type ViaCEP struct {
	BaseURL string
	Client  *http.Client
}

func NewViaCEP() *ViaCEP {
	return &ViaCEP{BaseURL: ViaCEPBaseURL, Client: defaultClient}
}

func (v *ViaCEP) Name() string { return "ViaCEP" }

type viaCEPResponse struct {
	CEP         string `json:"cep"`
	Logradouro  string `json:"logradouro"`
	Complemento string `json:"complemento"`
	Bairro      string `json:"bairro"`
	Localidade  string `json:"localidade"`
	UF          string `json:"uf"`
	// deixei como any porque a ViaCEP já mandou esse campo como "true" (string)
	// e como true (bool) dependendo da versão
	Erro any `json:"erro"`
}

func (v *ViaCEP) Fetch(ctx context.Context, cep string) (Address, error) {
	url := fmt.Sprintf("%s/%s/json/", v.BaseURL, cep)

	var body viaCEPResponse
	status, err := getJSON(ctx, v.Client, url, &body)
	switch {
	case err != nil:
		return Address{}, err
	case status != http.StatusOK:
		return Address{}, fmt.Errorf("status inesperado: %d", status)
	// pegadinha da ViaCEP: CEP que não existe vem com status 200 e {"erro":"true"}
	case isErroFlag(body.Erro):
		return Address{}, ErrCEPNaoEncontrado
	}

	return Address{
		CEP:         body.CEP,
		Logradouro:  body.Logradouro,
		Complemento: body.Complemento,
		Bairro:      body.Bairro,
		Cidade:      body.Localidade,
		UF:          body.UF,
	}, nil
}

func isErroFlag(v any) bool {
	switch e := v.(type) {
	case bool:
		return e
	case string:
		return strings.EqualFold(e, "true")
	default:
		return false
	}
}
