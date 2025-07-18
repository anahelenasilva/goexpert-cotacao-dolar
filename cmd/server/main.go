package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/anahelenadasilva/goexpert-cotacao-dolar/entities"
	_ "github.com/mattn/go-sqlite3"
)

const (
	EXCHANGE_RATE_API_TIMEOUT = 200 * time.Millisecond
	DATABASE_TIMEOUT          = 10 * time.Millisecond
	EXCHANGE_RATE_URL         = "https://economia.awesomeapi.com.br/json/last/USD-BRL"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", HomeHandler)
	mux.HandleFunc("/cotacao", ExchangeRateHandler)

	err := initDatabase()
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	err = http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}
}

func HomeHandler(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Write([]byte("Welcome to the Exchange Rate API!"))
}

func ExchangeRateHandler(responseWriter http.ResponseWriter, request *http.Request) {
	// Timeout para chamar a API de cotação
	apiCtx, apiCancel := context.WithTimeout(request.Context(), EXCHANGE_RATE_API_TIMEOUT)
	defer apiCancel()

	// Criar requisição com contexto
	req, err := http.NewRequestWithContext(apiCtx, "GET", EXCHANGE_RATE_URL, nil)
	if err != nil {
		log.Println("Failed to create request:", err)
		http.Error(responseWriter, "Failed to create request", http.StatusInternalServerError)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if apiCtx.Err() == context.DeadlineExceeded {
			log.Println("API request timeout exceeded (200ms)")
			http.Error(responseWriter, "API request timeout", http.StatusRequestTimeout)
			return
		}
		log.Println("Failed to fetch exchange rate:", err)
		http.Error(responseWriter, "Failed to fetch exchange rate", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	response, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Failed to read exchange rate response:", err)
		http.Error(responseWriter, "Failed to read exchange rate response", http.StatusInternalServerError)
		return
	}

	var exchangeRateResponse entities.ExchangeRateResponse
	err = json.Unmarshal(response, &exchangeRateResponse)
	if err != nil {
		log.Println("Failed to parse exchange rate response:", err)
		http.Error(responseWriter, "Failed to parse exchange rate response", http.StatusInternalServerError)
		return
	}

	db, err := getDatabaseConnection()
	if err != nil {
		log.Println("Database connection error:", err)
		http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	err = insertExchangeRate(db, exchangeRateResponse)
	if err != nil {
		log.Println("Database insertion error:", err)
		http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	json.NewEncoder(responseWriter).Encode(exchangeRateResponse.Usdbrl)
}

func getDatabaseConnection() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./exchange_rate.db")

	if err != nil {
		msg := "Failed to connect to database"
		log.Println(msg, err)
		return nil, errors.New(msg)
	}

	return db, nil
}

func initDatabase() error {
	db, err := getDatabaseConnection()

	if err != nil {
		return err
	}

	defer db.Close()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS exchange_rate (code TEXT, name TEXT, bid TEXT, timestamp TEXT)")
	if err != nil {
		msg := "Failed to create table"
		log.Println(msg, err)

		return errors.New(msg)
	}

	return nil
}

func insertExchangeRate(db *sql.DB, exchangeRateResponse entities.ExchangeRateResponse) error {
	statement, err := db.Prepare("INSERT INTO exchange_rate (code, bid, name, timestamp) VALUES (?, ?, ?, ?)")
	if err != nil {
		msg := "Failed to prepare statement for database"
		log.Println(msg, err)
		return errors.New(msg)
	}
	defer statement.Close()

	// Timeout para persistir dados no banco
	dbCtx, dbCancel := context.WithTimeout(context.Background(), DATABASE_TIMEOUT)
	defer dbCancel()

	_, err = statement.ExecContext(dbCtx, exchangeRateResponse.Usdbrl.Code, exchangeRateResponse.Usdbrl.Bid, exchangeRateResponse.Usdbrl.Name, exchangeRateResponse.Usdbrl.Timestamp)
	if err != nil {
		if dbCtx.Err() == context.DeadlineExceeded {
			msg := "Database insertion timeout exceeded (10ms)"
			log.Println(msg)
			return errors.New(msg)
		}
		msg := "Failed to insert exchange rate into database"
		log.Println(msg, err)
		return errors.New(msg)
	}
	return nil
}
