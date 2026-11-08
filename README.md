# Desafio Multithreading

Desafio da pós de Go. O programa consulta um CEP em duas APIs ao mesmo tempo e
mostra a resposta da que chegar primeiro, descartando a outra. Se nenhuma
responder em 1 segundo, ele avisa que deu timeout.

APIs usadas:

- BrasilAPI: https://brasilapi.com.br/api/cep/v1/{cep}
- ViaCEP: http://viacep.com.br/ws/{cep}/json/

## Requisitos

Go 1.22 ou mais novo e acesso à internet. A única dependência externa é o
testify, que só é usado nos testes. O `go mod download` resolve.

## Como rodar

```
go run ./cmd/buscacep 01153-000
```

O CEP pode vir com ou sem máscara, tanto faz: `01153-000`, `01.153-000` e
`01153000` dão no mesmo.

A saída é assim:

```
Consultando o CEP 01153-000 em 2 APIs simultaneamente (timeout de 1s)...

API vencedora: BrasilAPI (respondeu em 534ms)
-----------------------------------
CEP.........: 01153-000
Logradouro..: Rua Vitorino Carmilo
Bairro......: Barra Funda
Cidade......: São Paulo
UF..........: SP
```

Se rodar algumas vezes seguidas dá pra ver o vencedor mudando entre BrasilAPI e
ViaCEP, porque depende de qual responde mais rápido naquele momento.

Para gerar o binário:

```
go build -o buscacep ./cmd/buscacep
./buscacep 01153-000
```

No Windows o arquivo sai como `buscacep.exe`.

## Como testar

Os testes ficam em `internal/cep/cep_test.go` e não precisam de internet, porque
as duas APIs são simuladas com `httptest`:

```
go test ./...
go test -v ./internal/cep/
```

Eles cobrem a validação do CEP, a leitura do JSON de cada API (incluindo o caso
chato da ViaCEP, que responde 200 com `{"erro": "true"}` quando o CEP não existe)
e a corrida em si: o mais rápido ganhando, uma API falhando sem atrapalhar a
outra, o timeout e o caso das duas falharem.

Se tiver o gcc instalado dá pra rodar com o detector de concorrência também:

```
go test -race ./...
```

### Testando o timeout

Deixei uma flag `-timeout` justamente pra conseguir provocar o erro sem ter que
derrubar a internet. Com um prazo curto nenhuma das duas responde a tempo:

```
go run ./cmd/buscacep -timeout=1ms 01153-000
```

```
erro: timeout: nenhuma API respondeu em 1ms
```

O padrão da flag é 1s, que é o valor pedido no enunciado. Ela precisa vir antes
do CEP, senão o pacote `flag` trata como argumento comum.

### Outros erros

```
go run ./cmd/buscacep 00000000     # CEP que não existe
go run ./cmd/buscacep abc123       # CEP inválido
go run ./cmd/buscacep              # sem passar CEP
```

Nos três casos a mensagem vai pro stderr e o programa sai com código 1.

## Como funciona

A parte que interessa está em `internal/cep/race.go`.

O `main` cria um contexto com `context.WithTimeout` de 1 segundo e passa pra
função `Race`. Ela dispara uma goroutine pra cada API e cada uma escreve o
resultado num channel. Um `select` dentro de um loop fica esperando: se vier
resposta boa do channel, a função retorna na hora e a outra goroutine é
descartada; se o `ctx.Done()` fechar primeiro, é timeout.

Alguns detalhes que valem comentar:

O channel tem buffer do tamanho do número de APIs. Sem isso a goroutine perdedora
ficaria travada pra sempre tentando escrever num channel que ninguém mais lê,
vazando goroutine.

O contexto que a `Race` usa é um filho criado com `WithCancel`, e o `cancel` é
chamado no defer. Assim, no momento em que alguém ganha, a requisição HTTP da
perdedora é cancelada de verdade em vez de continuar rodando. Isso funciona
porque as requisições são montadas com `http.NewRequestWithContext`.

Erro não ganha a corrida. Se uma API responder rápido mas com erro (CEP
inexistente, 500, JSON quebrado), o erro é guardado e o loop continua esperando a
outra. Só se as duas falharem é que os erros são juntados e devolvidos.

O timeout é da operação inteira, não de cada requisição separada, já que as duas
goroutines compartilham o mesmo contexto.

## Estrutura

```
cmd/buscacep/main.go     entrada do programa, flag e formatação da saída
internal/cep/cep.go      struct Address, interface Provider e validação do CEP
internal/cep/http.go     função auxiliar de GET + decode do JSON
internal/cep/brasilapi.go
internal/cep/viacep.go
internal/cep/race.go     a corrida (goroutines, channel e select)
internal/cep/cep_test.go
```

As duas APIs devolvem JSON com nomes de campo diferentes (`street` x
`logradouro`, `city` x `localidade`, e por aí vai), então cada uma tem seu
arquivo e converte pra mesma struct `Address`. Quem chama a `Race` nem sabe qual
das duas respondeu.

Como as duas implementam a interface `Provider`, dá pra colocar uma terceira API
só criando o arquivo dela e adicionando na lista em `cmd/buscacep/main.go`. A
corrida já funciona com qualquer quantidade.
