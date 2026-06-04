# geogo

API HTTP simples em Go para consultar dados GeoIP (MaxMind MMDB) e decidir se um IP é permitido com base em regras de continente/país/cidade.

## O que este serviço faz

- Faz lookup de um endereço IP em um banco MaxMind MMDB (ex.: GeoLite2-City).
- Expõe endpoints HTTP para:
  - Saúde do serviço (`/healthz`)
  - Informações de GeoIP (`/info/{IP}`)
  - Checagem de acesso baseada em allowlist (`/access/{IP}`)
- Não faz chamadas externas em runtime: toda consulta é local no arquivo `.mmdb`.

## Stack e estrutura

- Linguagem: Go (módulo em [go.mod](file:///Users/pedromol/projetos/geogo/go.mod))
- Lookup GeoIP: `github.com/oschwald/maxminddb-golang/v2`
- Pacotes principais:
  - [main.go](file:///Users/pedromol/projetos/geogo/main.go): bootstrap, HTTP server, carga de config e MMDB.
  - [internal/config](file:///Users/pedromol/projetos/geogo/internal/config/config.go): flags/env vars.
  - [internal/api](file:///Users/pedromol/projetos/geogo/internal/api/api.go): roteamento e handlers.
  - [internal/geoip](file:///Users/pedromol/projetos/geogo/internal/geoip/geoip.go): decodificação do registro City.
  - [internal/allow](file:///Users/pedromol/projetos/geogo/internal/allow/allow.go): parsing e aplicação das regras.

## Requisitos

- Go conforme o campo `go` do módulo (ver [go.mod](file:///Users/pedromol/projetos/geogo/go.mod)).
- Um arquivo MMDB do MaxMind (ex.: `GeoLite2-City.mmdb`).

## Como rodar localmente

1) Baixe/obtenha um `.mmdb` (por exemplo, GeoLite2-City) e salve em algum caminho local.

2) Rode o serviço apontando para esse arquivo:

```bash
export GEOIP_DB_PATH=/caminho/para/GeoLite2-City.mmdb
go run .
```

O serviço inicia em `:8080` por padrão e loga `listening on :8080`.

### Flags (CLI)

As flags são lidas em [config.Load](file:///Users/pedromol/projetos/geogo/internal/config/config.go#L43-L86).

- `-addr` (default `:8080`): endereço do servidor HTTP.
- `-db` (default `./GeoLite2-City.mmdb`): caminho do arquivo `.mmdb`.
  - Se `-db` não for passado, o serviço tenta `GEOIP_DB_PATH`.
  - Se nenhum dos dois estiver disponível e `./GeoLite2-City.mmdb` não existir, o processo falha ao iniciar.

Exemplo:

```bash
go run . -addr :8080 -db ./GeoLite2-City.mmdb
```

## Configuração (env vars)

### Banco GeoIP

- `GEOIP_DB_PATH`: caminho do arquivo `.mmdb` (usado quando `-db` não é passado).

### Regras de allowlist

As regras são opcionais e podem ser combinadas. Se um grupo não for definido, ele não restringe (ou seja, não participa da decisão).

- `ALLOWED_CONTINENT`: lista CSV de continentes permitidos.
- `ALLOWED_COUNTRY`: lista CSV de países permitidos.
- `ALLOWED_CITY`: lista CSV de cidades permitidas.

Formato:

- CSV separado por vírgula.
- Comparação case-insensitive (o código normaliza para minúsculas).
- Os nomes vêm do MaxMind em inglês (`names["en"]`), então os valores devem bater com o dataset (ex.: `United States`, `Brazil`, `Europe`).

Exemplo (permitir apenas Brasil e Portugal):

```bash
export ALLOWED_COUNTRY="Brazil,Portugal"
```

### Healthcheck

O healthcheck faz um lookup em um IP conhecido e valida se o país retornado bate com o esperado.

- `HEALTHCHECK_IP` (default `8.8.8.8`): IP consultado em `/healthz`.
- `HEALTHCHECK_EXPECTED_COUNTRY` (default `United States`): país esperado (nome em inglês conforme o MMDB).

Se o lookup falhar ou o país não bater, `/healthz` retorna `500`.

## API

Todos os endpoints aceitam apenas `GET`. Respostas são JSON (`application/json; charset=utf-8`) exceto alguns casos de erro do healthcheck que respondem apenas com status.

### `GET /healthz`

Faz lookup do `HEALTHCHECK_IP` e valida se o `country` é igual a `HEALTHCHECK_EXPECTED_COUNTRY`.

Exemplo:

```bash
curl -sS http://localhost:8080/healthz | jq .
```

Resposta (exemplo):

```json
{
  "ip": "8.8.8.8",
  "timestamp": "2026-01-01T00:00:00Z",
  "continent": "North America",
  "country": "United States",
  "city": "Mountain View",
  "latitude": 37.4056,
  "longitude": -122.0775,
  "time_zone": "America/Los_Angeles"
}
```

### `GET /info/{IP}`

Retorna informações GeoIP do IP consultado.

Exemplo:

```bash
curl -sS http://localhost:8080/info/8.8.8.8 | jq .
```

Campos:

- `ip`: IP em texto (como foi passado na URL).
- `timestamp`: UTC.
- `continent`, `country`, `city`: nomes em inglês conforme MMDB (podem vir vazios).
- `latitude`, `longitude`, `time_zone`: dados de localização (podem vir vazios/zero conforme o MMDB).

Erros:

- `400` se o IP for inválido ou ausente.
- `500` se o lookup no MMDB falhar.

### `GET /access/{IP}`

Faz lookup do IP e aplica as regras de allowlist. Se não passar, retorna `403`.

Exemplo:

```bash
curl -i http://localhost:8080/access/8.8.8.8
```

Respostas:

- `200`:

```json
{
  "ip": "8.8.8.8",
  "allowed": true
}
```

- `403`:

```json
{
  "error": "ip not allowed"
}
```

Erros:

- `400` se o IP for inválido ou ausente.
- `500` se o lookup no MMDB falhar.

## Docker

O [Dockerfile](file:///Users/pedromol/projetos/geogo/Dockerfile) gera uma imagem `scratch` contendo:

- Binário estático (`/main`)
- Certificados CA
- `GeoLite2-City.mmdb` baixado durante o build

Durante o build, o arquivo MMDB é baixado via BuildKit Secret `secret_url`:

- O secret deve conter uma URL que devolva o banco (pode ser `.mmdb` direto ou um `.gz`; o Dockerfile detecta gzip e descompacta).

### Build local (com BuildKit)

Crie um arquivo com a URL do banco:

```bash
printf '%s' 'https://exemplo.com/GeoLite2-City.mmdb' > secret_url.txt
```

Build:

```bash
DOCKER_BUILDKIT=1 docker build \
  --secret id=secret_url,src=secret_url.txt \
  -t geogo:local .
```

Run:

```bash
docker run --rm -p 8080:8080 geogo:local
```

## Kubernetes

O manifesto em [deploy.yaml](file:///Users/pedromol/projetos/geogo/deploy.yaml) cria:

- Namespace `geogo`
- Deployment `geogo` (1 réplica)
- Service `ClusterIP` na porta 8080
- NetworkPolicy negando todo egress (`egress: []`)

Aplicar:

```bash
kubectl apply -f deploy.yaml
```

Observações:

- A imagem usada é `pedromol/geogo:latest`.
- O container expõe `containerPort: 8080` e as probes chamam `/healthz`.
- Com egress bloqueado, o pod não consegue sair para a rede; como o runtime só usa o MMDB local, isso é compatível com a proposta do serviço.

## CI (build/push da imagem)

O workflow em [build.yml](file:///Users/pedromol/projetos/geogo/.github/workflows/build.yml):

- Faz build multi-arch (amd64/arm64/armv7)
- Faz push para Docker Hub
- Injeta o secret `secret_url` como `secret_url=${{ secrets.URL }}` para baixar o MMDB durante o build

Secrets esperados no repositório:

- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN`
- `URL` (fonte do MMDB ou do `.gz`)

## Desenvolvimento

### Testes

Atualmente não há testes automatizados. Ainda assim, o comando padrão passa:

```bash
go test ./...
```

### Dicas de troubleshooting

- Erro ao iniciar “missing GeoIP database path”: defina `GEOIP_DB_PATH`, passe `-db` ou coloque `./GeoLite2-City.mmdb` no diretório de execução.
- `/healthz` retornando `500`: ajuste `HEALTHCHECK_IP` e/ou `HEALTHCHECK_EXPECTED_COUNTRY` para valores coerentes com o seu MMDB.
- `/access/{ip}` sempre `403`: valide se `ALLOWED_*` está correto e se os nomes batem com os retornados pelo endpoint `/info/{ip}` (em inglês).
