package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	searchworker "tp-final-sdp/internal/worker"
)

const (
	defaultAPIBaseURL = "http://localhost:8080"
	defaultCharset    = "abcdefghijklmnopqrstuvwxyz"
	defaultChunkSize  = 1000
	maxTerminalRanges = 1_000_000
	pollInterval      = time.Second
)

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
	apiBaseURL := strings.TrimRight(env("API_BASE_URL", defaultAPIBaseURL), "/")
	client := &http.Client{Timeout: 5 * time.Second}
	reader := bufio.NewReader(os.Stdin)

	for {
		clearScreen()
		printPorts(apiBaseURL)

		req, err := readCreateJobRequest(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			fmt.Printf("\nError: %v\n", err)
			waitEnter(reader)
			continue
		}

		createResp, err := createJob(context.Background(), client, apiBaseURL, req)
		if err != nil {
			fmt.Printf("\nNo se pudo crear el job: %v\n", err)
			waitEnter(reader)
			continue
		}

		pollJob(context.Background(), client, apiBaseURL, createResp)

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
		return fmt.Errorf("espacio de busqueda invalido: %w", err)
	}
	totalRanges := ((totalCandidates - 1) / chunkSize) + 1
	if totalRanges > maxTerminalRanges {
		return fmt.Errorf("espacio de busqueda demasiado grande: %d candidatos y %d rangos; usa una password mas corta, un charset mas chico o un chunk size mas grande", totalCandidates, totalRanges)
	}
	return nil
}

func createJob(ctx context.Context, client *http.Client, apiBaseURL string, request createJobRequest) (createJobResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return createJobResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/jobs", bytes.NewReader(body))
	if err != nil {
		return createJobResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

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

func pollJob(ctx context.Context, client *http.Client, apiBaseURL string, createResp createJobResponse) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		jobResp, err := getJob(ctx, client, apiBaseURL, createResp.JobID)
		clearScreen()
		printPorts(apiBaseURL)
		printJob(createResp, jobResp, err)

		if err == nil && isFinalStatus(jobResp.Status) {
			return
		}

		<-ticker.C
	}
}

func getJob(ctx context.Context, client *http.Client, apiBaseURL string, jobID string) (getJobResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL+"/jobs/"+jobID, nil)
	if err != nil {
		return getJobResponse{}, err
	}

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

func printPorts(apiBaseURL string) {
	fmt.Println("TP Final SDP - Terminal")
	fmt.Println()
	fmt.Println("Puertos:")
	fmt.Printf("  API Gateway:       %s\n", apiBaseURL)
	fmt.Println("  Nginx:             http://localhost:8088")
	fmt.Println("  Scheduler metrics: http://localhost:9100/metrics")
	fmt.Println("  Prometheus:        http://localhost:9091")
	fmt.Println("  Grafana:           http://localhost:3000")
	fmt.Println("  Redis:             localhost:6379")
	fmt.Println("  PostgreSQL:        localhost:5432")
	fmt.Println()
}

func printJob(createResp createJobResponse, jobResp getJobResponse, err error) {
	fmt.Println("Resultado:")
	fmt.Printf("  Job ID:            %s\n", createResp.JobID)
	fmt.Printf("  Candidatos:        %d\n", createResp.TotalCandidates)
	fmt.Printf("  Rangos totales:    %d\n", createResp.TotalRanges)
	if err != nil {
		fmt.Printf("  Estado:            error consultando job: %v\n", err)
		return
	}

	fmt.Printf("  Estado:            %s\n", jobResp.Status)
	fmt.Printf("  Charset:           %s\n", jobResp.Charset)
	fmt.Printf("  Longitud:          %d\n", jobResp.MinLength)
	fmt.Printf("  Rangos completos:  %d/%d\n", jobResp.CompletedRanges, jobResp.TotalRanges)
	if jobResp.Plaintext != "" {
		fmt.Printf("  Password hallada:  %s\n", jobResp.Plaintext)
	}
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
