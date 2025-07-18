package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/anahelenadasilva/goexpert-cotacao-dolar/entities"
)

const SERVER_TIMEOUT = 300 * time.Millisecond
const SERVER_URL = "http://localhost:8080/"
const COTACAO_ENDPOINT = "cotacao"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), SERVER_TIMEOUT)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s%s", SERVER_URL, COTACAO_ENDPOINT), nil)
	if err != nil {
		log.Println("Failed to create request:", err)
		panic(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Println("Server request timeout exceeded (300ms)")
		} else {
			log.Println("Failed to fetch exchange rate from server:", err)
		}
		panic(err)
	}
	defer resp.Body.Close()

	response, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Failed to read server response:", err)
		panic(err)
	}

	exchangeRateResponse := entities.Usdbrl{}

	err = json.Unmarshal(response, &exchangeRateResponse)
	if err != nil {
		log.Println("Failed to parse server response:", err)
		panic(err)
	}

	err = saveToFile(string(exchangeRateResponse.Bid))
	if err != nil {
		log.Println("Failed to save to file:", err)
		panic(err)
	}

	fmt.Println("Response:", string(exchangeRateResponse.Bid))
}

func saveToFile(bid string) error {
	file, err := os.OpenFile("cotacao.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("Error creating file:", err.Error())
		return err
	}

	defer file.Close()

	_, err = file.WriteString(fmt.Sprintf("Dólar: %s\n", bid))
	if err != nil {
		fmt.Println("Error writing to file:", err.Error())
		return err
	}

	fmt.Println("Data saved to file successfully")
	return nil
}
