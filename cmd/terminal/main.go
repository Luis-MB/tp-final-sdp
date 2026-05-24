package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	searchworker "tp-final-sdp/internal/worker"
)

const (
	defaultAPIBaseURL      = "http://localhost:8080"
	defaultSchedulerHealth = "http://localhost:9100/healthz"
	defaultCharset         = "abcdefghijklmnopqrstuvwxyz"
	defaultChunkSize       = 1000
	maxTerminalRanges      = 1_000_000
	pollInterval           = time.Second
)

type terminalConfig struct {
	APIBaseURL       string
	APIToken         string
	SchedulerHealth  string
	PostgresEndpoint string
}

type createJobRequest struct {
	Password  string `json:"password"`
	Charset   string `json:"charset"`
	ChunkSize uint64 `json:"chunk_size"`
}

type createJobResponse struct {
	JobID           string `json:"job_id"`
	TotalCandidates uint64 `json:"total_candidates"`
	TotalRanges     uint64 `json:"total_ranges"`
}

type getJobResponse struct {
	JobID           string `json:"job_id"`
	TargetHash      string `json:"target_hash"`
	Charset         string `json:"charset"`
	MinLength       uint32 `json:"min_length"`
	MaxLength       uint32 `json:"max_length"`
	Status          string `json:"status"`
	Plaintext       string `json:"plaintext"`
	CompletedRanges uint64 `json:"completed_ranges"`
	TotalRanges     uint64 `json:"total_ranges"`
	TotalCandidates uint64 `json:"total_candidates"`
}

func main() {
	cfg := terminalConfig{
		APIBaseURL:       strings.TrimRight(env("API_BASE_URL", defaultAPIBaseURL), "/"),
		APIToken:         env("API_TOKEN", ""),
		SchedulerHealth:  env("SCHEDULER_HEALTH_URL", defaultSchedulerHealth),
		PostgresEndpoint: env("POSTGRES_ENDPOINT", "localhost:15432"),
	}
	client := &http.Client{Timeout: 5 * time.Second}
	reader := bufio.NewReader(os.Stdin)

	for {
		clearScreen()
		printHeader(context.Background(), client, cfg)

		req, err := readCreateJobRequest(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			fmt.Printf("\nError: %v\n", err)
			waitEnter(reader)
			continue
		}

		createResp, err := createJob(context.Background(), client, cfg.APIBaseURL, cfg.APIToken, req)
		if err != nil {
			fmt.Printf("\nNo se pudo crear el job: %v\n", err)
			waitEnter(reader)
			continue
		}

		pollJob(context.Background(), client, cfg, createResp)

		fmt.Print("\nCrear otro job? [s/N]: ")
		answer, err := readLine(reader)
		if err != nil {
			return
		}
		if strings.ToLower(answer) != "s" {
			return
		}
	}
}

func readCreateJobRequest(reader *bufio.Reader) (createJobRequest, error) {
	password, err := promptRequired(reader, "Password a hashear y buscar")
	if err != nil {
		return createJobRequest{}, err
	}

	charset, err := promptWithDefault(reader, "Charset", defaultCharset)
	if err != nil {
		return createJobRequest{}, err
	}

	chunkSizeText, err := promptWithDefault(reader, "Chunk size", strconv.FormatUint(defaultChunkSize, 10))
	if err != nil {
		return createJobRequest{}, err
	}
	chunkSize, err := strconv.ParseUint(chunkSizeText, 10, 64)
	if err != nil || chunkSize == 0 {
		return createJobRequest{}, fmt.Errorf("chunk size invalido")
	}
	if err := validateSearchSpace(password, charset, chunkSize); err != nil {
		return createJobRequest{}, err
	}

	return createJobRequest{
		Password:  password,
		Charset:   charset,
		ChunkSize: chunkSize,
	}, nil
}

func validateSearchSpace(password string, charset string, chunkSize uint64) error {
	passwordLength := uint32(len(password))
	totalCandidates, err := searchworker.TotalCandidates(charset, passwordLength, passwordLength)
	if err != nil {
		return fmt.Errorf("espacio de busqueda invalido: %w; reduce el charset o usa una password mas corta", err)
	}
	totalRanges := ((totalCandidates - 1) / chunkSize) + 1
	if totalRanges > maxTerminalRanges {
		return fmt.Errorf("espacio de busqueda demasiado grande: %d candidatos y %d rangos; usa una password mas corta, un charset mas chico o un chunk size mas grande", totalCandidates, totalRanges)
	}
	return nil
}

func createJob(ctx context.Context, client *http.Client, apiBaseURL string, apiToken string, request createJobRequest) (createJobResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return createJobResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/jobs", bytes.NewReader(body))
	if err != nil {
		return createJobResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	setAuthHeader(httpReq, apiToken)

	resp, err := client.Do(httpReq)
	if err != nil {
		return createJobResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return createJobResponse{}, responseError(resp)
	}

	var createResp createJobResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return createJobResponse{}, err
	}
	return createResp, nil
}

func pollJob(ctx context.Context, client *http.Client, cfg terminalConfig, createResp createJobResponse) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	startedAt := time.Now()
	var lastCompleted uint64
	lastUpdated := startedAt

	for {
		jobResp, err := getJob(ctx, client, cfg.APIBaseURL, cfg.APIToken, createResp.JobID)
		now := time.Now()
		completedDelta := uint64(0)
		if err == nil && jobResp.CompletedRanges >= lastCompleted {
			completedDelta = jobResp.CompletedRanges - lastCompleted
			lastCompleted = jobResp.CompletedRanges
			lastUpdated = now
		}

		clearScreen()
		printHeader(ctx, client, cfg)
		printJob(createResp, jobResp, err, now.Sub(startedAt), completedDelta, now.Sub(lastUpdated))

		if err == nil && isFinalStatus(jobResp.Status) {
			return
		}

		<-ticker.C
	}
}

func getJob(ctx context.Context, client *http.Client, apiBaseURL string, apiToken string, jobID string) (getJobResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL+"/jobs/"+jobID, nil)
	if err != nil {
		return getJobResponse{}, err
	}
	setAuthHeader(httpReq, apiToken)

	resp, err := client.Do(httpReq)
	if err != nil {
		return getJobResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return getJobResponse{}, responseError(resp)
	}

	var jobResp getJobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jobResp); err != nil {
		return getJobResponse{}, err
	}
	return jobResp, nil
}

func setAuthHeader(req *http.Request, apiToken string) {
	if apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+apiToken)
	}
}

func printHeader(ctx context.Context, client *http.Client, cfg terminalConfig) {
	fmt.Println("TP Final SDP - Terminal")
	fmt.Println()
	fmt.Println("Puertos:")
	fmt.Printf("  API Gateway:       %s\n", cfg.APIBaseURL)
	fmt.Println("  Nginx:             http://localhost:8088")
	fmt.Println("  Scheduler metrics: http://localhost:9100/metrics")
	fmt.Println("  Prometheus:        http://localhost:9091")
	fmt.Println("  Grafana:           http://localhost:3000")
	fmt.Println("  Adminer:           http://localhost:8081")
	fmt.Printf("  PostgreSQL:        %s\n", cfg.PostgresEndpoint)
	fmt.Println()
	fmt.Println("Estado:")
	fmt.Printf("  API health:        %s\n", healthStatus(ctx, client, cfg.APIBaseURL+"/healthz", ""))
	fmt.Printf("  Scheduler health:  %s\n", healthStatus(ctx, client, cfg.SchedulerHealth, ""))
	postgresStatus := tcpStatus(cfg.PostgresEndpoint)
	fmt.Printf("  PostgreSQL health: %s\n", postgresStatus)
	if postgresStatus == "ok" {
		fmt.Println("  Persistencia:      PostgreSQL habilitado")
	} else {
		fmt.Println("  Persistencia:      PostgreSQL no disponible")
	}
	if cfg.APIToken == "" {
		fmt.Println("  API token:         desactivado")
	} else {
		fmt.Println("  API token:         activado")
	}
	fmt.Println()
}

func printJob(createResp createJobResponse, jobResp getJobResponse, err error, elapsed time.Duration, completedDelta uint64, staleFor time.Duration) {
	fmt.Println("Resultado:")
	fmt.Printf("  Job ID:            %s\n", createResp.JobID)
	fmt.Printf("  Candidatos:        %d\n", createResp.TotalCandidates)
	fmt.Printf("  Rangos totales:    %d\n", createResp.TotalRanges)
	fmt.Printf("  Tiempo:            %s\n", elapsed.Round(time.Second))
	if err != nil {
		fmt.Printf("  Estado:            error consultando job: %v\n", err)
		return
	}

	fmt.Printf("  Estado:            %s\n", jobResp.Status)
	fmt.Printf("  Charset:           %s\n", jobResp.Charset)
	fmt.Printf("  Longitud:          %d\n", jobResp.MinLength)
	fmt.Printf("  Rangos completos:  %d/%d\n", jobResp.CompletedRanges, jobResp.TotalRanges)
	fmt.Printf("  Progreso:          %s %.1f%%\n", progressBar(jobResp.CompletedRanges, jobResp.TotalRanges, 24), percent(jobResp.CompletedRanges, jobResp.TotalRanges))
	fmt.Printf("  Ultimo cambio:     hace %s\n", staleFor.Round(time.Second))
	fmt.Printf("  Ritmo actual:      +%d rangos/s\n", completedDelta)
	if jobResp.Plaintext != "" {
		fmt.Printf("  Password hallada:  %s\n", jobResp.Plaintext)
	}
}

func healthStatus(ctx context.Context, client *http.Client, url string, apiToken string) string {
	reqCtx, cancel := context.WithTimeout(ctx, 700*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return "error"
	}
	setAuthHeader(req, apiToken)
	resp, err := client.Do(req)
	if err != nil {
		return "no disponible"
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "ok"
	}
	return fmt.Sprintf("HTTP %d", resp.StatusCode)
}

func tcpStatus(address string) string {
	conn, err := net.DialTimeout("tcp", address, 700*time.Millisecond)
	if err != nil {
		return "no disponible"
	}
	_ = conn.Close()
	return "ok"
}

func progressBar(done, total uint64, width int) string {
	if total == 0 {
		return strings.Repeat("-", width)
	}
	filled := int((done * uint64(width)) / total)
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + "]"
}

func percent(done, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return (float64(done) / float64(total)) * 100
}

func promptRequired(reader *bufio.Reader, label string) (string, error) {
	value, err := promptWithDefault(reader, label, "")
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("%s es requerido", label)
	}
	return value, nil
}

func promptWithDefault(reader *bufio.Reader, label string, fallback string) (string, error) {
	if fallback == "" {
		fmt.Printf("%s: ", label)
	} else {
		fmt.Printf("%s [%s]: ", label, fallback)
	}

	value, err := readLine(reader)
	if err != nil {
		return "", err
	}
	if value == "" {
		return fallback, nil
	}
	return value, nil
}

func readLine(reader *bufio.Reader) (string, error) {
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if errors.Is(err, io.EOF) && value == "" {
		return "", io.EOF
	}
	return strings.TrimSpace(value), nil
}

func waitEnter(reader *bufio.Reader) {
	fmt.Print("\nEnter para continuar...")
	_, _ = readLine(reader)
}

func responseError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, message)
}

func isFinalStatus(status string) bool {
	return status == "found" || status == "completed" || status == "failed"
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
