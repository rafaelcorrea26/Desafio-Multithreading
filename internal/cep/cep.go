package cep

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// usado quando a API responde certinho mas não conhece aquele CEP
var ErrCEPNaoEncontrado = errors.New("cep não encontrado")

// struct única de endereço. Cada API devolve o JSON com nomes de campo
// diferentes, então cada uma converte pra cá.
type Address struct {
	CEP         string
	Logradouro  string
	Complemento string
	Bairro      string
	Cidade      string
	UF          string
}

// implementa fmt.Stringer, por isso o fmt.Println(addr) no main já sai formatado
func (a Address) String() string {
	linhas := []string{
		fmt.Sprintf("CEP.........: %s", a.CEP),
		fmt.Sprintf("Logradouro..: %s", a.Logradouro),
	}
	// a BrasilAPI não devolve complemento
	if a.Complemento != "" {
		linhas = append(linhas, fmt.Sprintf("Complemento.: %s", a.Complemento))
	}
	linhas = append(linhas,
		fmt.Sprintf("Bairro......: %s", a.Bairro),
		fmt.Sprintf("Cidade......: %s", a.Cidade),
		fmt.Sprintf("UF..........: %s", a.UF),
	)
	return strings.Join(linhas, "\n")
}

// interface que as duas APIs implementam
type Provider interface {
	Name() string
	Fetch(ctx context.Context, cep string) (Address, error)
}

// tira a máscara e confere se sobraram 8 dígitos
func Normalize(raw string) (string, error) {
	var digitos strings.Builder
	for _, r := range raw {
		switch {
		case r >= '0' && r <= '9':
			digitos.WriteRune(r)
		case r == '.' || r == '-' || r == ' ':
			// separador, ignora
		default:
			return "", fmt.Errorf("cep inválido: %q contém caractere não numérico %q", raw, r)
		}
	}

	cep := digitos.String()
	if len(cep) != 8 {
		return "", fmt.Errorf("cep inválido: %q deve ter 8 dígitos, tem %d", raw, len(cep))
	}
	return cep, nil
}

// 01153000 vira 01153-000
func Format(cep string) string {
	if len(cep) == 8 {
		return cep[:5] + "-" + cep[5:]
	}
	return cep
}
