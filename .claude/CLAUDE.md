# Data Governance Service - Guía de Desarrollo

## 🎯 Contexto del Proyecto

Sistema de clasificación de auxiliares empresariales que usa LLM para categorizar automáticamente líneas de detalle. El sistema procesa archivos ZIP conteniendo múltiples formatos (Excel/CSV/JSON/JSONL/NDJSON), los limpia, deduplica, clasifica con GPT-4, y permite validación manual con refinamiento iterativo usando prompts customizables.

## 🏗️ Arquitectura Monorepo

```
data-governance-service/          # Monorepo root
├── backend/                      # Go backend (FOCO ACTUAL)
├── frontend/                     # Svelte + Skeleton (FUTURO)
└── shared/                       # Código compartido
```

## 🚀 Stack Tecnológico

### Backend (Go) - PRIORIDAD ACTUAL
- **Framework Web**: **Gin** (balance perfecto entre performance y madurez)
- **Task Queue**: Asynq (Redis-based, simple, suficiente para nuestro volumen)
- **Database**: PostgreSQL con pgx/v5 + sqlc (type-safe queries)
- **Cache**: Redis con go-redis/v9
- **LLM**: OpenAI GPT-4o-mini via sashabaranov/go-openai
- **File Processing**:
  - archive/zip (ZIP extraction)
  - excelize/v2 (Excel/XLSX)
  - encoding/csv (CSV)
  - encoding/json (JSON/JSONL/NDJSON)
- **Validation**: go-playground/validator/v10
- **WebSocket**: github.com/gorilla/websocket
- **Testing**:
  - testify (assertions)
  - mockery (mocks generation)
  - testcontainers-go (integration tests)

### Frontend (Futuro)
- **Framework**: SvelteKit
- **UI Library**: Skeleton UI
- **State Management**: Svelte stores
- **API Client**: Native fetch o SvelteKit's load functions
- **Type Safety**: TypeScript

## 📁 Estructura Backend Go

```
backend/
├── cmd/
│   ├── api/                    # API REST server
│   │   └── main.go
│   └── worker/                  # Worker para procesamiento async
│       └── main.go
├── internal/
│   ├── api/                    # Capa HTTP
│   │   ├── handlers/           # HTTP handlers
│   │   │   ├── upload.go       # Upload de archivos
│   │   │   ├── cleaning.go     # Limpieza/dedup
│   │   │   ├── llm.go         # Procesamiento LLM
│   │   │   ├── validation.go   # Validación/refinement
│   │   │   ├── batch.go       # Batch processing
│   │   │   └── session.go     # Gestión de sesiones
│   │   ├── middleware/
│   │   │   ├── cors.go
│   │   │   ├── logger.go
│   │   │   └── recovery.go
│   │   └── websocket/
│   │       └── progress.go     # Updates de progreso
│   ├── core/                   # Dominio/Negocio (agnóstico de infra)
│   │   ├── domain/             # Entidades y value objects
│   │   │   ├── batch.go
│   │   │   ├── classification.go
│   │   │   ├── validation.go
│   │   │   └── session.go
│   │   ├── ports/              # Interfaces (hexagonal)
│   │   │   ├── repositories.go
│   │   │   ├── services.go
│   │   │   └── external.go
│   │   └── services/           # Lógica de negocio
│   │       ├── upload_service.go
│   │       ├── cleaning_service.go
│   │       ├── llm_service.go
│   │       ├── validation_service.go
│   │       ├── batch_service.go
│   │       └── metrics_service.go
│   ├── infrastructure/         # Implementaciones externas
│   │   ├── database/
│   │   │   ├── postgres.go    # Conexión
│   │   │   └── repositories/  # Implementación de repos
│   │   │       ├── batch_repo.go
│   │   │       └── session_repo.go
│   │   ├── cache/
│   │   │   └── redis.go
│   │   ├── storage/
│   │   │   └── local.go       # File system
│   │   ├── llm/
│   │   │   └── openai.go      # Cliente OpenAI
│   │   └── queue/
│   │       └── asynq.go        # Task queue
│   └── pkg/                    # Utilidades compartidas
│       ├── config/             # Configuración
│       ├── logger/             # Logging
│       ├── errors/             # Error handling
│       └── utils/              # Helpers
├── migrations/                  # SQL migrations
├── configs/                     # Config files
│   ├── dev.yaml
│   └── prod.yaml
└── tests/                       # Integration tests
```

## 🔄 Mapeo de Servicios Python → Go

### Servicios Core y su Responsabilidad

| Python Service | Go Service | Responsabilidad |
|----------------|------------|-----------------|
| `llm/classifier.py` | `llm_service.go` | Clasificación con OpenAI, chunking, retry logic |
| `batch/batch_processor.py` | `batch_service.go` | Procesamiento multi-archivo |
| `batch/batch_state_manager.py` | `session_repo.go` | Estado en Redis + PostgreSQL |
| `cleaning/deduplication.py` | `cleaning_service.go` | Limpieza y deduplicación |
| `validation/sampling.py` | `validation_service.go` | Muestreo estratificado |
| `validation/iteration_tracking.py` | `iteration_service.go` | Tracking de iteraciones |
| `metrics/*` | `metrics_service.go` | Captura y comparación de métricas |

## 📝 Implementación Crítica - LLM Service

### Estructura del LLM Service (LA MÁS CRÍTICA)

```go
// internal/core/services/llm_service.go

type LLMService struct {
    client     LLMClient
    repository BatchRepository
    cache      Cache
    logger     *slog.Logger
    config     LLMConfig
}

type LLMConfig struct {
    Model            string        // "gpt-4o-mini"
    ChunkSize        int          // Configurable desde ENV: LLM_DISTRIBUTED_CHUNK_SIZE
    MaxWorkers       int          // Configurable desde ENV: LLM_MAX_WORKERS
    MaxRetries       int          // 3
    ConcurrencyLimit int          // Configurable desde ENV: LLM_CONCURRENCY_LIMIT
    Timeout          time.Duration // 30s per request
}

// Método principal - CRÍTICO: mantener relación 1:1 input/output
func (s *LLMService) ClassifyBatch(ctx context.Context, records []Record) (*ClassificationResult, error) {
    // 1. Detectar campos clean* dinámicamente
    cleanFields := detectCleanFields(records[0])

    // 2. Agregar _row_index para tracking (CRÍTICO!)
    for i := range records {
        records[i].RowIndex = i
    }

    // 3. Dividir en chunks
    chunks := createChunks(records, s.config.ChunkSize)

    // 4. Procesar en paralelo con semaphore
    sem := make(chan struct{}, s.config.ConcurrencyLimit)
    results := make([]ChunkResult, len(chunks))
    var wg sync.WaitGroup

    for i, chunk := range chunks {
        wg.Add(1)
        go func(idx int, c []Record) {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()

            results[idx] = s.processChunk(ctx, c, cleanFields)
        }(i, chunk)
    }

    wg.Wait()

    // 5. Merge results usando _row_index (NO por contenido!)
    return s.mergeResults(records, results)
}

// Procesamiento de chunk individual
func (s *LLMService) processChunk(ctx context.Context, chunk []Record, fields []string, promptID string) ChunkResult {
    retries := 0

    for retries < s.config.MaxRetries {
        // Obtener prompt customizado del usuario
        prompt, err := s.promptService.GetPromptForProcessing(ctx, promptID, chunk, fields)
        if err != nil {
            return ChunkResult{Success: false, Error: fmt.Sprintf("failed to get prompt: %v", err)}
        }

        // Call OpenAI
        response, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
            Model: s.config.Model,
            Messages: []openai.ChatMessage{
                {Role: "user", Content: prompt},
            },
        })

        if err != nil {
            retries++
            time.Sleep(time.Duration(math.Pow(2, float64(retries))) * time.Second)
            continue
        }

        // Parse response
        var result LLMResponse
        if err := json.Unmarshal([]byte(response.Choices[0].Message.Content), &result); err != nil {
            retries++
            continue
        }

        // CRÍTICO: Validar count match
        if len(result.Results) != len(chunk) {
            result = s.fixCountMismatch(result, chunk, fields[0])
        }

        return ChunkResult{Success: true, Data: result}
    }

    return ChunkResult{Success: false, Error: "max retries exceeded"}
}

// Fix cuando LLM retorna número incorrecto de resultados
func (s *LLMService) fixCountMismatch(result LLMResponse, chunk []Record, primaryField string) LLMResponse {
    // Crear mapping por descripción
    resultMap := make(map[string]ClassificationItem)
    for _, item := range result.Results {
        key := normalizeString(item.Description)
        resultMap[key] = item
    }

    // Match con input, llenar faltantes
    fixed := make([]ClassificationItem, 0, len(chunk))
    for _, record := range chunk {
        key := normalizeString(record.Data[primaryField].(string))
        if item, exists := resultMap[key]; exists {
            fixed = append(fixed, item)
        } else {
            // Default para faltantes
            fixed = append(fixed, ClassificationItem{
                Description: record.Data[primaryField].(string),
                Category:    "Indeterminado",
                Reason:      "No classification returned",
                Score:       -1,
            })
        }
    }

    return LLMResponse{Results: fixed}
}
```

### Sistema Multi-Proveedor LLM

```go
// internal/core/ports/llm.go

type LLMProvider string

const (
    ProviderOpenAI LLMProvider = "openai"
    ProviderGemini LLMProvider = "gemini"
)

// Interface común para todos los proveedores
type LLMClient interface {
    CreateCompletion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    GetModel() string
    GetProvider() LLMProvider
    ValidateAPIKey() error
}

type CompletionRequest struct {
    Model       string
    Messages    []Message
    Temperature float32
    MaxTokens   int
}

type Message struct {
    Role    string
    Content string
}

type CompletionResponse struct {
    ID      string
    Content string
    Usage   TokenUsage
}

type TokenUsage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}
```

### Implementación OpenAI Provider
```go
// internal/infrastructure/llm/openai_provider.go

type OpenAIProvider struct {
    client *openai.Client
    config OpenAIConfig
    logger *slog.Logger
}

type OpenAIConfig struct {
    APIKey      string
    Model       string // gpt-4o-mini, gpt-4, etc
    MaxRetries  int
    Timeout     time.Duration
}

func NewOpenAIProvider(config OpenAIConfig, logger *slog.Logger) *OpenAIProvider {
    client := openai.NewClient(config.APIKey)
    return &OpenAIProvider{
        client: client,
        config: config,
        logger: logger,
    }
}

func (p *OpenAIProvider) CreateCompletion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
    messages := make([]openai.ChatCompletionMessage, len(req.Messages))
    for i, msg := range req.Messages {
        messages[i] = openai.ChatCompletionMessage{
            Role:    msg.Role,
            Content: msg.Content,
        }
    }

    resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:       req.Model,
        Messages:    messages,
        Temperature: req.Temperature,
        MaxTokens:   req.MaxTokens,
    })

    if err != nil {
        return nil, fmt.Errorf("openai completion failed: %w", err)
    }

    return &CompletionResponse{
        ID:      resp.ID,
        Content: resp.Choices[0].Message.Content,
        Usage: TokenUsage{
            PromptTokens:     resp.Usage.PromptTokens,
            CompletionTokens: resp.Usage.CompletionTokens,
            TotalTokens:      resp.Usage.TotalTokens,
        },
    }, nil
}
```

### Implementación Gemini Provider
```go
// internal/infrastructure/llm/gemini_provider.go

type GeminiProvider struct {
    client *genai.Client
    model  *genai.GenerativeModel
    config GeminiConfig
    logger *slog.Logger
}

type GeminiConfig struct {
    APIKey      string
    Model       string // gemini-pro, gemini-1.5-pro, etc
    MaxRetries  int
    Timeout     time.Duration
}

func NewGeminiProvider(ctx context.Context, config GeminiConfig, logger *slog.Logger) (*GeminiProvider, error) {
    client, err := genai.NewClient(ctx, option.WithAPIKey(config.APIKey))
    if err != nil {
        return nil, fmt.Errorf("failed to create gemini client: %w", err)
    }

    model := client.GenerativeModel(config.Model)

    return &GeminiProvider{
        client: client,
        model:  model,
        config: config,
        logger: logger,
    }, nil
}

func (p *GeminiProvider) CreateCompletion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
    // Configurar parámetros del modelo
    p.model.SetTemperature(req.Temperature)
    p.model.SetMaxOutputTokens(int32(req.MaxTokens))

    // Construir prompt
    prompt := p.buildPrompt(req.Messages)

    // Generar respuesta
    resp, err := p.model.GenerateContent(ctx, genai.Text(prompt))
    if err != nil {
        return nil, fmt.Errorf("gemini completion failed: %w", err)
    }

    // Extraer contenido
    var content string
    for _, candidate := range resp.Candidates {
        if candidate.Content != nil {
            for _, part := range candidate.Content.Parts {
                content += fmt.Sprintf("%v", part)
            }
        }
    }

    return &CompletionResponse{
        ID:      resp.PromptFeedback.BlockReasonMessage,
        Content: content,
        Usage:   p.extractUsage(resp),
    }, nil
}
```

### Factory para Proveedores
```go
// internal/infrastructure/llm/factory.go

type LLMProviderFactory struct {
    providers map[LLMProvider]LLMClient
    logger    *slog.Logger
}

func NewLLMProviderFactory(logger *slog.Logger) *LLMProviderFactory {
    return &LLMProviderFactory{
        providers: make(map[LLMProvider]LLMClient),
        logger:    logger,
    }
}

func (f *LLMProviderFactory) RegisterProvider(provider LLMProvider, client LLMClient) {
    f.providers[provider] = client
}

func (f *LLMProviderFactory) GetProvider(provider LLMProvider) (LLMClient, error) {
    client, exists := f.providers[provider]
    if !exists {
        return nil, fmt.Errorf("provider %s not registered", provider)
    }
    return client, nil
}

// Setup inicial de proveedores desde config
func (f *LLMProviderFactory) InitializeProviders(config *config.LLMConfig) error {
    // OpenAI
    if config.OpenAI.Enabled {
        openaiProvider := NewOpenAIProvider(OpenAIConfig{
            APIKey:     config.OpenAI.APIKey,
            Model:      config.OpenAI.Model,
            MaxRetries: 3,
            Timeout:    30 * time.Second,
        }, f.logger)
        f.RegisterProvider(ProviderOpenAI, openaiProvider)
    }

    // Gemini
    if config.Gemini.Enabled {
        geminiProvider, err := NewGeminiProvider(context.Background(), GeminiConfig{
            APIKey:     config.Gemini.APIKey,
            Model:      config.Gemini.Model,
            MaxRetries: 3,
            Timeout:    30 * time.Second,
        }, f.logger)
        if err != nil {
            return fmt.Errorf("failed to initialize Gemini: %w", err)
        }
        f.RegisterProvider(ProviderGemini, geminiProvider)
    }

    return nil
}
```

## 📦 Sistema de Procesamiento de ZIPs (PRIORIDAD)

### Estructura del ZIP Processor
```go
// internal/core/services/zip_processor.go

type ZIPProcessor struct {
    logger         *slog.Logger
    cleaningService CleaningService
    config         ZIPConfig
}

type ZIPConfig struct {
    MaxFileSize      int64  // Max size por archivo (MB)
    SupportedFormats []string // [".csv", ".xlsx", ".json", ".jsonl", ".ndjson"]
    TempDir         string // "/tmp/uploads"
    StreamingChunkSize int  // 1000 registros
}

// Procesar ZIP con múltiples archivos
func (z *ZIPProcessor) ProcessZIP(ctx context.Context, zipPath string) (*ProcessedData, error) {
    reader, err := zip.OpenReader(zipPath)
    if err != nil {
        return nil, fmt.Errorf("failed to open zip: %w", err)
    }
    defer reader.Close()

    var allRecords []map[string]interface{}
    fileStats := make(map[string]FileStats)

    // Procesar cada archivo en el ZIP
    for _, file := range reader.File {
        if !z.isSupported(file.Name) {
            z.logger.Warn("skipping unsupported file",
                slog.String("file", file.Name))
            continue
        }

        records, stats, err := z.processFile(ctx, file)
        if err != nil {
            z.logger.Error("failed to process file",
                slog.String("file", file.Name),
                slog.Error(err))
            continue
        }

        allRecords = append(allRecords, records...)
        fileStats[file.Name] = stats
    }

    return &ProcessedData{
        Records: allRecords,
        Stats:   fileStats,
        TotalRecords: len(allRecords),
    }, nil
}

// Procesar archivo individual con streaming
func (z *ZIPProcessor) processFile(ctx context.Context, file *zip.File) ([]map[string]interface{}, FileStats, error) {
    rc, err := file.Open()
    if err != nil {
        return nil, FileStats{}, err
    }
    defer rc.Close()

    ext := strings.ToLower(filepath.Ext(file.Name))

    switch ext {
    case ".csv":
        return z.processCSV(rc)
    case ".xlsx":
        return z.processExcel(rc, file.UncompressedSize64)
    case ".json":
        return z.processJSON(rc)
    case ".jsonl", ".ndjson":
        return z.processJSONLines(rc)
    default:
        return nil, FileStats{}, fmt.Errorf("unsupported format: %s", ext)
    }
}

// Streaming para JSONL/NDJSON
func (z *ZIPProcessor) processJSONLines(r io.Reader) ([]map[string]interface{}, FileStats, error) {
    scanner := bufio.NewScanner(r)
    scanner.Buffer(make([]byte, 64*1024), 1024*1024) // Buffer de 1MB

    var records []map[string]interface{}
    batch := make([]map[string]interface{}, 0, z.config.StreamingChunkSize)
    lineNum := 0

    for scanner.Scan() {
        lineNum++
        var record map[string]interface{}

        if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
            // Log error pero continuar con siguiente línea
            continue
        }

        batch = append(batch, record)

        // Procesar batch cuando se llena
        if len(batch) >= z.config.StreamingChunkSize {
            cleaned := z.cleaningService.CleanBatch(batch)
            records = append(records, cleaned...)
            batch = batch[:0] // Reuse slice
        }
    }

    // Procesar batch restante
    if len(batch) > 0 {
        cleaned := z.cleaningService.CleanBatch(batch)
        records = append(records, cleaned...)
    }

    return records, FileStats{
        TotalLines: lineNum,
        ProcessedRecords: len(records),
        Format: "JSONL",
    }, scanner.Err()
}
```

## 🎨 Sistema de Gestión de Prompts Customizables

### Modelo de Prompts
```go
// internal/core/domain/prompt.go

type Prompt struct {
    ID          string    `json:"id" db:"id"`
    Name        string    `json:"name" db:"name"`
    Label       string    `json:"label" db:"label"`
    Template    string    `json:"template" db:"template"`
    Categories  []Category `json:"categories" db:"categories"`
    IsDefault   bool      `json:"is_default" db:"is_default"`
    CreatedBy   string    `json:"created_by" db:"created_by"`
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
    Version     int       `json:"version" db:"version"`
}

type Category struct {
    ID          int    `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Priority    int    `json:"priority"`
    Keywords    []string `json:"keywords"`
}

// Repository para prompts
type PromptRepository interface {
    Create(ctx context.Context, prompt *Prompt) error
    GetByID(ctx context.Context, id string) (*Prompt, error)
    GetByLabel(ctx context.Context, label string) (*Prompt, error)
    List(ctx context.Context, userID string) ([]*Prompt, error)
    Update(ctx context.Context, prompt *Prompt) error
    SetDefault(ctx context.Context, id string) error
    Delete(ctx context.Context, id string) error
    GetHistory(ctx context.Context, promptID string) ([]*PromptVersion, error)
}
```

### Service de Prompts
```go
// internal/core/services/prompt_service.go

type PromptService struct {
    repo   PromptRepository
    cache  Cache
    logger *slog.Logger
}

// Crear prompt customizado
func (s *PromptService) CreateCustomPrompt(ctx context.Context, req CreatePromptRequest) (*Prompt, error) {
    prompt := &Prompt{
        ID:        uuid.New().String(),
        Name:      req.Name,
        Label:     req.Label,
        Template:  req.Template,
        Categories: req.Categories,
        CreatedBy: req.UserID,
        CreatedAt: time.Now(),
        Version:   1,
    }

    // Validar template
    if err := s.validateTemplate(prompt.Template); err != nil {
        return nil, fmt.Errorf("invalid template: %w", err)
    }

    // Guardar en DB
    if err := s.repo.Create(ctx, prompt); err != nil {
        return nil, err
    }

    // Cache para acceso rápido
    s.cache.Set(ctx, fmt.Sprintf("prompt:%s", prompt.ID), prompt, 1*time.Hour)

    return prompt, nil
}

// Obtener prompt para procesamiento
func (s *PromptService) GetPromptForProcessing(ctx context.Context, promptID string) (string, error) {
    // Check cache first
    cacheKey := fmt.Sprintf("prompt:compiled:%s", promptID)
    if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
        return cached.(string), nil
    }

    prompt, err := s.repo.GetByID(ctx, promptID)
    if err != nil {
        // Fallback to default
        prompt, err = s.getDefaultPrompt(ctx)
        if err != nil {
            return "", err
        }
    }

    // Compile template with categories
    compiled := s.compilePrompt(prompt)

    // Cache compiled prompt
    s.cache.Set(ctx, cacheKey, compiled, 1*time.Hour)

    return compiled, nil
}

// Compilar prompt con categorías
func (s *PromptService) compilePrompt(prompt *Prompt) string {
    var sb strings.Builder

    // Header
    sb.WriteString(prompt.Template)
    sb.WriteString("\n\nCATEGORÍAS Y REGLAS:\n")

    // Categories
    for i, cat := range prompt.Categories {
        sb.WriteString(fmt.Sprintf("%d. %s - %s\n",
            i+1, cat.Name, cat.Description))

        if len(cat.Keywords) > 0 {
            sb.WriteString(fmt.Sprintf("   Keywords: %s\n",
                strings.Join(cat.Keywords, ", ")))
        }
    }

    // Footer with mandatory instructions
    sb.WriteString("\n\nCRÍTICO: Debes retornar EXACTAMENTE el mismo número de resultados que entradas recibidas.")

    return sb.String()
}
```

### API para Gestión de Prompts
```go
// internal/api/handlers/prompt_handler.go

// GET /api/v1/prompts
func (h *PromptHandler) List(c *gin.Context) {
    userID := c.GetString("user_id") // From auth middleware

    prompts, err := h.service.ListUserPrompts(c.Request.Context(), userID)
    if err != nil {
        c.JSON(500, gin.H{"error": "failed to list prompts"})
        return
    }

    c.JSON(200, prompts)
}

// POST /api/v1/prompts
func (h *PromptHandler) Create(c *gin.Context) {
    var req CreatePromptRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    req.UserID = c.GetString("user_id")

    prompt, err := h.service.CreateCustomPrompt(c.Request.Context(), req)
    if err != nil {
        c.JSON(500, gin.H{"error": "failed to create prompt"})
        return
    }

    c.JSON(201, prompt)
}

// PUT /api/v1/prompts/:id/default
func (h *PromptHandler) SetDefault(c *gin.Context) {
    promptID := c.Param("id")

    if err := h.service.SetAsDefault(c.Request.Context(), promptID); err != nil {
        c.JSON(500, gin.H{"error": "failed to set default"})
        return
    }

    c.JSON(200, gin.H{"message": "prompt set as default"})
}
```

## 🐳 Docker Compose - Stack Completo

### docker-compose.yml
```yaml
version: '3.8'

services:
  # PostgreSQL Database
  postgres:
    image: postgres:15-alpine
    container_name: dgs-postgres
    environment:
      POSTGRES_USER: ${DB_USER:-admin}
      POSTGRES_PASSWORD: ${DB_PASSWORD:-changeme123}
      POSTGRES_DB: ${DB_NAME:-datagovernance}
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U admin"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - dgs-network

  # Redis Cache & Queue
  redis:
    image: redis:7-alpine
    container_name: dgs-redis
    command: redis-server --appendonly yes
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - dgs-network

  # API Service
  api:
    build:
      context: ./backend
      dockerfile: Dockerfile
      target: api
    container_name: dgs-api
    environment:
      - ENV=development
      - DB_HOST=postgres
      - DB_USER=${DB_USER:-admin}
      - DB_PASSWORD=${DB_PASSWORD:-changeme123}
      - DB_NAME=${DB_NAME:-datagovernance}
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - PORT=8080
    ports:
      - "8080:8080"
    volumes:
      - ./backend:/app
      - /tmp/uploads:/tmp/uploads
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    networks:
      - dgs-network
    command: air -c .air.toml # Hot reload para desarrollo

  # Worker Service
  worker:
    build:
      context: ./backend
      dockerfile: Dockerfile
      target: worker
    container_name: dgs-worker
    environment:
      - ENV=development
      - DB_HOST=postgres
      - DB_USER=${DB_USER:-admin}
      - DB_PASSWORD=${DB_PASSWORD:-changeme123}
      - DB_NAME=${DB_NAME:-datagovernance}
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    volumes:
      - ./backend:/app
      - /tmp/uploads:/tmp/uploads
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    networks:
      - dgs-network
    command: air -c .air.worker.toml

  # Asynq Monitor (Web UI para queues)
  asynqmon:
    image: hibiken/asynqmon:latest
    container_name: dgs-asynqmon
    ports:
      - "8081:8080"
    environment:
      - REDIS_ADDR=redis:6379
    depends_on:
      - redis
    networks:
      - dgs-network

  # pgAdmin (opcional, para desarrollo)
  pgadmin:
    image: dpage/pgadmin4:latest
    container_name: dgs-pgadmin
    environment:
      PGADMIN_DEFAULT_EMAIL: ${PGADMIN_EMAIL:-admin@example.com}
      PGADMIN_DEFAULT_PASSWORD: ${PGADMIN_PASSWORD:-admin}
    ports:
      - "5050:80"
    volumes:
      - pgadmin_data:/var/lib/pgadmin
    networks:
      - dgs-network
    profiles:
      - tools

volumes:
  postgres_data:
  redis_data:
  pgadmin_data:

networks:
  dgs-network:
    driver: bridge
```

### Dockerfile Multi-stage
```dockerfile
# backend/Dockerfile

# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o api cmd/api/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o worker cmd/worker/main.go

# API Runtime
FROM alpine:latest AS api
RUN apk --no-cache add ca-certificates
WORKDIR /app

COPY --from=builder /build/api .
COPY --from=builder /build/configs ./configs
COPY --from=builder /build/migrations ./migrations

EXPOSE 8080
CMD ["./api"]

# Worker Runtime
FROM alpine:latest AS worker
RUN apk --no-cache add ca-certificates
WORKDIR /app

COPY --from=builder /build/worker .
COPY --from=builder /build/configs ./configs

CMD ["./worker"]
```

## 🔌 API Endpoints Principales (Actualizado con Gin)

### 1. Upload Endpoint (ZIP Support)
```go
// POST /api/v1/uploads
func (h *UploadHandler) Upload(c *gin.Context) {
    file, err := c.FormFile("file")
    if err != nil {
        return fiber.NewError(fiber.StatusBadRequest, "file required")
    }

    // Validate file type
    if !isValidFileType(file.Filename) {
        return fiber.NewError(fiber.StatusBadRequest, "invalid file type")
    }

    // Save to temp
    uploadID := uuid.New().String()
    tempPath := filepath.Join("/tmp", fmt.Sprintf("upload_%s", uploadID))

    if err := c.SaveFile(file, tempPath); err != nil {
        return fiber.NewError(fiber.StatusInternalServerError, "failed to save file")
    }

    // Parse and get columns
    columns, rowCount, err := h.service.ParseFileHeaders(tempPath)
    if err != nil {
        return fiber.NewError(fiber.StatusBadRequest, "failed to parse file")
    }

    return c.JSON(fiber.Map{
        "upload_id": uploadID,
        "columns": columns,
        "row_count": rowCount,
        "status": "ready",
    })
}
```

### 2. Cleaning Pipeline
```go
// POST /api/v1/cleaning/process
func (h *CleaningHandler) Process(c *fiber.Ctx) error {
    var req CleaningRequest
    if err := c.BodyParser(&req); err != nil {
        return fiber.NewError(fiber.StatusBadRequest, "invalid request")
    }

    // Load data
    data, err := h.service.LoadUpload(req.UploadID)
    if err != nil {
        return fiber.NewError(fiber.StatusNotFound, "upload not found")
    }

    // Clean columns
    cleaned := h.service.CleanColumns(data, req.ColumnsToClean, req.RefineryType)

    // Deduplicate
    deduplicated, stats := h.service.Deduplicate(cleaned, req.DeduplicationStrategy)

    // Generate LLM input
    llmInput := h.service.GenerateLLMInput(deduplicated)

    // Save results
    batchID := uuid.New().String()
    h.service.SaveBatch(batchID, deduplicated, llmInput)

    return c.JSON(fiber.Map{
        "batch_id": batchID,
        "stats": stats,
        "total_entries": len(deduplicated),
    })
}
```

### 3. LLM Processing (Async)
```go
// POST /api/v1/llm/process
func (h *LLMHandler) Process(c *fiber.Ctx) error {
    var req LLMRequest
    if err := c.BodyParser(&req); err != nil {
        return fiber.NewError(fiber.StatusBadRequest, "invalid request")
    }

    // Create async task
    task := asynq.NewTask("llm:process", map[string]interface{}{
        "upload_id": req.UploadID,
        "prompt": req.Prompt,
        "chunk_size": req.ChunkSize,
    })

    info, err := h.queue.Enqueue(task,
        asynq.Queue("high-priority"),
        asynq.MaxRetry(3),
    )

    if err != nil {
        return fiber.NewError(fiber.StatusInternalServerError, "failed to queue task")
    }

    return c.JSON(fiber.Map{
        "task_id": info.ID,
        "status": "processing",
        "message": "LLM processing started",
    })
}
```

## 🛡️ Sistema de Resiliencia y Recuperación

### Checkpointing para Batches
```go
// internal/core/services/checkpoint_service.go

type CheckpointService struct {
    repo   CheckpointRepository
    cache  *redis.Client
    logger *slog.Logger
}

type BatchCheckpoint struct {
    ID              string                `json:"id" db:"id"`
    BatchID         string                `json:"batch_id" db:"batch_id"`
    WorkerID        string                `json:"worker_id" db:"worker_id"`
    TotalChunks     int                   `json:"total_chunks" db:"total_chunks"`
    ProcessedChunks []int                 `json:"processed_chunks" db:"processed_chunks"`
    FailedChunks    []ChunkFailure        `json:"failed_chunks" db:"failed_chunks"`
    State           map[string]interface{} `json:"state" db:"state"`
    LastHeartbeat   time.Time             `json:"last_heartbeat" db:"last_heartbeat"`
    CreatedAt       time.Time             `json:"created_at" db:"created_at"`
    UpdatedAt       time.Time             `json:"updated_at" db:"updated_at"`
}

type ChunkFailure struct {
    ChunkID     int    `json:"chunk_id"`
    Error       string `json:"error"`
    RetryCount  int    `json:"retry_count"`
    LastAttempt time.Time `json:"last_attempt"`
}

// Guardar checkpoint después de cada chunk procesado
func (s *CheckpointService) SaveProgress(ctx context.Context, batchID string, chunkID int, data interface{}) error {
    checkpoint, err := s.getOrCreateCheckpoint(ctx, batchID)
    if err != nil {
        return err
    }

    // Actualizar chunks procesados
    checkpoint.ProcessedChunks = append(checkpoint.ProcessedChunks, chunkID)
    checkpoint.LastHeartbeat = time.Now()

    // Guardar estado parcial si es necesario
    if data != nil {
        checkpoint.State[fmt.Sprintf("chunk_%d", chunkID)] = data
    }

    // Persistir en DB (transaccional)
    return s.repo.Update(ctx, checkpoint)
}

// Recuperar trabajo incompleto
func (s *CheckpointService) RecoverIncompleteWork(ctx context.Context) ([]BatchCheckpoint, error) {
    // Buscar checkpoints con heartbeat > 5 minutos
    staleTime := time.Now().Add(-5 * time.Minute)

    checkpoints, err := s.repo.GetStaleCheckpoints(ctx, staleTime)
    if err != nil {
        return nil, err
    }

    // Para cada checkpoint stale, determinar qué chunks faltan
    for i, cp := range checkpoints {
        remainingChunks := s.calculateRemainingChunks(cp)
        checkpoints[i].State["remaining_chunks"] = remainingChunks
    }

    return checkpoints, nil
}

// Calcular chunks que faltan procesar
func (s *CheckpointService) calculateRemainingChunks(cp BatchCheckpoint) []int {
    processed := make(map[int]bool)
    for _, chunk := range cp.ProcessedChunks {
        processed[chunk] = true
    }

    var remaining []int
    for i := 0; i < cp.TotalChunks; i++ {
        if !processed[i] {
            remaining = append(remaining, i)
        }
    }

    return remaining
}
```

### Worker con Health Checks y Graceful Shutdown
```go
// cmd/worker/main.go

type Worker struct {
    srv            *asynq.Server
    mux            *asynq.ServeMux
    checkpointSvc  *CheckpointService
    healthChecker  *HealthChecker
    shutdownCh     chan os.Signal
    workerID       string
    logger         *slog.Logger
}

func NewWorker(cfg *config.Config) (*Worker, error) {
    // Generar Worker ID único
    workerID := fmt.Sprintf("worker-%s-%d", hostname(), os.Getpid())

    // Redis para Asynq
    redis := asynq.RedisClientOpt{
        Addr:         cfg.RedisAddr,
        DB:           cfg.RedisDB,
        DialTimeout:  10 * time.Second,
        ReadTimeout:  2 * time.Minute,
        WriteTimeout: 2 * time.Minute,
    }

    // Configurar servidor con resiliencia
    srv := asynq.NewServer(redis, asynq.Config{
        Concurrency: cfg.WorkerConcurrency,
        Queues: map[string]int{
            "critical":      6,
            "high-priority": 3,
            "default":       1,
        },

        // Configuración de resiliencia
        StrictPriority:   true,
        HealthCheckFunc:  healthCheckFunc,
        HealthCheckInterval: 20 * time.Second,

        // Retry configuration
        RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
            return time.Duration(n) * 2 * time.Second // Exponential backoff
        },

        // Error handling
        ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
            logger.Error("task failed",
                slog.String("type", task.Type()),
                slog.String("payload", string(task.Payload())),
                slog.Error(err),
            )
        }),

        // Graceful shutdown timeout
        ShutdownTimeout: 25 * time.Second,
    })

    // Setup mux con middleware
    mux := asynq.NewServeMux()

    // Middleware para tracking y recovery
    mux.Use(recoveryMiddleware)
    mux.Use(checkpointMiddleware)
    mux.Use(metricsMiddleware)

    return &Worker{
        srv:           srv,
        mux:           mux,
        workerID:      workerID,
        shutdownCh:    make(chan os.Signal, 1),
        logger:        logger,
    }, nil
}

// Iniciar worker con manejo de señales
func (w *Worker) Start(ctx context.Context) error {
    // Registrar handlers
    w.registerHandlers()

    // Setup signal handling para graceful shutdown
    signal.Notify(w.shutdownCh, syscall.SIGTERM, syscall.SIGINT)

    // Iniciar health check endpoint
    go w.startHealthCheckServer()

    // Iniciar heartbeat para checkpoints
    go w.startHeartbeat(ctx)

    // Recuperar trabajo incompleto
    if err := w.recoverIncompleteWork(ctx); err != nil {
        w.logger.Error("failed to recover incomplete work", slog.Error(err))
    }

    // Run server en goroutine
    errCh := make(chan error, 1)
    go func() {
        w.logger.Info("starting worker", slog.String("id", w.workerID))
        if err := w.srv.Run(w.mux); err != nil {
            errCh <- err
        }
    }()

    // Wait for shutdown signal
    select {
    case <-w.shutdownCh:
        w.logger.Info("received shutdown signal, starting graceful shutdown")
        return w.gracefulShutdown(ctx)
    case err := <-errCh:
        return fmt.Errorf("server error: %w", err)
    }
}

// Graceful shutdown con checkpoint final
func (w *Worker) gracefulShutdown(ctx context.Context) error {
    w.logger.Info("starting graceful shutdown")

    // 1. Dejar de aceptar nuevas tareas
    w.srv.Shutdown()

    // 2. Esperar a que terminen las tareas actuales (con timeout)
    shutdownCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
    defer cancel()

    // 3. Guardar checkpoint final de tareas en progreso
    if err := w.saveInProgressCheckpoints(shutdownCtx); err != nil {
        w.logger.Error("failed to save final checkpoints", slog.Error(err))
    }

    // 4. Cerrar conexiones
    w.logger.Info("graceful shutdown completed")
    return nil
}

// Heartbeat para mantener checkpoints activos
func (w *Worker) startHeartbeat(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := w.updateHeartbeats(ctx); err != nil {
                w.logger.Error("heartbeat update failed", slog.Error(err))
            }
        }
    }
}

// Health check server
func (w *Worker) startHealthCheckServer() {
    mux := http.NewServeMux()

    // Liveness probe
    mux.HandleFunc("/health/live", func(rw http.ResponseWriter, r *http.Request) {
        rw.WriteHeader(http.StatusOK)
        json.NewEncoder(rw).Encode(map[string]string{
            "status": "alive",
            "worker_id": w.workerID,
        })
    })

    // Readiness probe
    mux.HandleFunc("/health/ready", func(rw http.ResponseWriter, r *http.Request) {
        if w.isReady() {
            rw.WriteHeader(http.StatusOK)
            json.NewEncoder(rw).Encode(map[string]interface{}{
                "status": "ready",
                "worker_id": w.workerID,
                "queue_info": w.srv.GetQueueInfo(),
            })
        } else {
            rw.WriteHeader(http.StatusServiceUnavailable)
            json.NewEncoder(rw).Encode(map[string]string{
                "status": "not_ready",
                "worker_id": w.workerID,
            })
        }
    })

    server := &http.Server{
        Addr:    ":8082",
        Handler: mux,
    }

    w.logger.Info("health check server started", slog.String("addr", ":8082"))
    if err := server.ListenAndServe(); err != nil {
        w.logger.Error("health check server error", slog.Error(err))
    }
}
```

### Middleware de Recuperación
```go
// internal/api/middleware/recovery.go

// Middleware para checkpoint automático
func checkpointMiddleware(h asynq.Handler) asynq.Handler {
    return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
        // Extraer batch ID del payload
        var payload map[string]interface{}
        json.Unmarshal(t.Payload(), &payload)
        batchID := payload["batch_id"].(string)

        // Crear checkpoint inicial
        checkpoint := &BatchCheckpoint{
            ID:       uuid.New().String(),
            BatchID:  batchID,
            WorkerID: ctx.Value("worker_id").(string),
            CreatedAt: time.Now(),
        }

        // Guardar en contexto
        ctx = context.WithValue(ctx, "checkpoint", checkpoint)

        // Ejecutar handler con recovery
        err := h.ProcessTask(ctx, t)

        // Actualizar checkpoint final
        if err != nil {
            checkpoint.State["error"] = err.Error()
            checkpoint.State["failed_at"] = time.Now()
        } else {
            checkpoint.State["completed_at"] = time.Now()
        }

        // Guardar checkpoint
        checkpointService.Save(ctx, checkpoint)

        return err
    })
}

// Middleware para recovery de panics
func recoveryMiddleware(h asynq.Handler) asynq.Handler {
    return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) (err error) {
        defer func() {
            if r := recover(); r != nil {
                // Log panic con stack trace
                logger.Error("panic recovered",
                    slog.Any("panic", r),
                    slog.String("task", t.Type()),
                    slog.String("stack", string(debug.Stack())),
                )

                // Guardar estado de panic en checkpoint
                if cp, ok := ctx.Value("checkpoint").(*BatchCheckpoint); ok {
                    cp.State["panic"] = fmt.Sprintf("%v", r)
                    cp.State["stack_trace"] = string(debug.Stack())
                    checkpointService.Save(ctx, cp)
                }

                // Retornar error para que Asynq pueda hacer retry
                err = fmt.Errorf("panic: %v", r)
            }
        }()

        return h.ProcessTask(ctx, t)
    })
}
```

### Sistema de Recuperación Automática
```go
// internal/core/services/recovery_service.go

type RecoveryService struct {
    checkpointSvc *CheckpointService
    queueClient   *asynq.Client
    logger        *slog.Logger
}

// Ejecutar en startup para recuperar trabajo perdido
func (r *RecoveryService) RecoverLostWork(ctx context.Context) error {
    // 1. Buscar checkpoints huérfanos (worker muerto)
    orphanedCheckpoints, err := r.checkpointSvc.GetOrphanedCheckpoints(ctx)
    if err != nil {
        return err
    }

    r.logger.Info("found orphaned checkpoints",
        slog.Int("count", len(orphanedCheckpoints)))

    // 2. Para cada checkpoint huérfano
    for _, cp := range orphanedCheckpoints {
        // Determinar qué chunks faltan
        remainingChunks := r.calculateRemainingWork(cp)

        // Re-encolar solo los chunks faltantes
        for _, chunkID := range remainingChunks {
            task := asynq.NewTask("process:chunk", map[string]interface{}{
                "batch_id":    cp.BatchID,
                "chunk_id":    chunkID,
                "retry_count": cp.State["retry_count"].(int) + 1,
                "resumed":     true,
                "original_worker": cp.WorkerID,
            })

            _, err := r.queueClient.Enqueue(task,
                asynq.Queue("high-priority"),
                asynq.MaxRetry(3),
                asynq.Unique(24*time.Hour), // Evitar duplicados
            )

            if err != nil {
                r.logger.Error("failed to re-queue chunk",
                    slog.Int("chunk_id", chunkID),
                    slog.Error(err))
            }
        }

        // Marcar checkpoint como recuperado
        cp.State["recovered_at"] = time.Now()
        cp.State["recovered_by"] = "recovery_service"
        r.checkpointSvc.Update(ctx, cp)
    }

    return nil
}

// Monitoreo continuo de workers
func (r *RecoveryService) MonitorWorkerHealth(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            r.checkWorkerHealth(ctx)
        }
    }
}

func (r *RecoveryService) checkWorkerHealth(ctx context.Context) {
    // Obtener lista de workers activos
    workers, err := r.getActiveWorkers(ctx)
    if err != nil {
        r.logger.Error("failed to get active workers", slog.Error(err))
        return
    }

    // Verificar heartbeats
    for _, worker := range workers {
        lastHeartbeat := worker.LastHeartbeat

        if time.Since(lastHeartbeat) > 2*time.Minute {
            r.logger.Warn("worker appears dead",
                slog.String("worker_id", worker.ID),
                slog.Time("last_heartbeat", lastHeartbeat))

            // Recuperar su trabajo
            if err := r.recoverWorkerTasks(ctx, worker.ID); err != nil {
                r.logger.Error("failed to recover worker tasks",
                    slog.String("worker_id", worker.ID),
                    slog.Error(err))
            }
        }
    }
}
```

## 🔄 Task Queue con Asynq (con Resiliencia)

### Setup del Worker Resiliente
```go
// cmd/worker/main.go

func main() {
    // Cargar configuración
    cfg := config.Load()

    // Crear worker con resiliencia
    worker, err := NewWorker(cfg)
    if err != nil {
        log.Fatal("failed to create worker:", err)
    }

    // Context con cancelación
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Iniciar recovery service en background
    recoveryService := recovery.NewService(cfg)
    go recoveryService.Start(ctx)

    // Iniciar worker
    if err := worker.Start(ctx); err != nil {
        log.Fatal("worker failed:", err)
    }
}

// Handler example
func HandleLLMProcess(ctx context.Context, t *asynq.Task) error {
    var payload map[string]interface{}
    if err := json.Unmarshal(t.Payload(), &payload); err != nil {
        return err
    }

    uploadID := payload["upload_id"].(string)

    // Load data
    data, err := loadLLMInput(uploadID)
    if err != nil {
        return err
    }

    // Process with LLM service
    results, err := llmService.ClassifyBatch(ctx, data)
    if err != nil {
        return err
    }

    // Save results
    return saveLLMResults(uploadID, results)
}
```

## 🧪 Testing Crítico

### Test del Count Mismatch Fix
```go
func TestLLMService_FixCountMismatch(t *testing.T) {
    service := NewLLMService(mockClient, logger)

    // Input con 3 records
    input := []Record{
        {Data: map[string]interface{}{"cleanLineDescription": "libro mental"}},
        {Data: map[string]interface{}{"cleanLineDescription": "revista pop"}},
        {Data: map[string]interface{}{"cleanLineDescription": "vinil display"}},
    }

    // LLM retorna solo 2 (falta uno)
    llmResponse := LLMResponse{
        Results: []ClassificationItem{
            {Description: "libro mental", Category: "Pop"},
            {Description: "revista pop", Category: "Publicidad"},
        },
    }

    fixed := service.fixCountMismatch(llmResponse, input, "cleanLineDescription")

    assert.Len(t, fixed.Results, 3, "debe tener 3 resultados")
    assert.Equal(t, "Indeterminado", fixed.Results[2].Category, "faltante debe ser Indeterminado")
    assert.Equal(t, -1.0, fixed.Results[2].Score, "score debe ser -1")
}
```

## 🚀 Comandos de Desarrollo

```bash
# Setup inicial
go mod init github.com/alejandroruanova/data-governance-service/backend
go mod tidy

# Run API server
go run cmd/api/main.go

# Run worker
go run cmd/worker/main.go

# Run tests
go test ./...

# Build
go build -o bin/api cmd/api/main.go
go build -o bin/worker cmd/worker/main.go

# Docker
docker-compose up -d

# Migrations
migrate -path migrations -database "postgresql://..." up
```

## 📊 Optimizaciones Clave vs Python

### 1. Streaming sin DataFrames
```go
// En lugar de cargar todo en memoria como pandas
func ProcessLargeCSV(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return err
    }
    defer file.Close()

    reader := csv.NewReader(file)

    // Process header
    header, err := reader.Read()

    // Stream records
    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }

        // Process individual record
        processRecord(record)
    }

    return nil
}
```

### 2. Concurrencia Real vs AsyncIO
```go
// Python: await asyncio.gather(*tasks) - single thread
// Go: real parallelism con goroutines

func ProcessChunksParallel(chunks [][]Record) []Result {
    results := make([]Result, len(chunks))
    var wg sync.WaitGroup

    for i, chunk := range chunks {
        wg.Add(1)
        go func(idx int, c []Record) {
            defer wg.Done()
            results[idx] = processChunk(c)
        }(i, chunk)
    }

    wg.Wait()
    return results
}
```

### 3. Memory Efficiency
```go
// Reuso de slices
batch := make([]Record, 0, 1000)
for _, record := range stream {
    batch = append(batch, record)

    if len(batch) >= 1000 {
        processBatch(batch)
        batch = batch[:0] // Reuse memory
    }
}
```

## 🎯 Próximos Pasos Inmediatos

1. **Setup inicial del backend**:
   - Crear estructura de carpetas
   - Inicializar go.mod
   - Instalar dependencias core

2. **Implementar primer endpoint**:
   - Upload handler
   - Parse de archivos
   - Respuesta compatible con frontend actual

3. **Conectar infraestructura**:
   - PostgreSQL con pgx
   - Redis para cache/queue
   - Setup de migrations

4. **LLM Service básico**:
   - Cliente OpenAI
   - Chunking logic
   - Count mismatch handling

¿Comenzamos con el setup inicial del backend en Go?