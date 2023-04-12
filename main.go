package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"golang.org/x/text/encoding/charmap"
)

type ValCurs struct {
	XMLName xml.Name `xml:"ValCurs"`
	Text    string   `xml:",chardata"`
	Date    string   `xml:"Date,attr"`
	Name    string   `xml:"name,attr"`
	Valute  []struct {
		Text     string `xml:",chardata"`
		ID       string `xml:"ID,attr"`
		NumCode  string `xml:"NumCode"`
		CharCode string `xml:"CharCode"`
		Nominal  string `xml:"Nominal"`
		Name     string `xml:"Name"`
		Value    string `xml:"Value"`
	} `xml:"Valute"`
}

type Currency struct {
	Min     float64
	MinDate string
	Max     float64
	MaxDate string
	Sum     float64
}

const cbrURL = "https://www.cbr.ru/scripts/XML_daily_eng.asp?date_req="
const period = 90

func decodeWindows1251(xmlData io.Reader) *ValCurs {
	d := xml.NewDecoder(xmlData)
	d.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		switch charset {
		case "windows-1251":
			return charmap.Windows1251.NewDecoder().Reader(input), nil
		default:
			return nil, fmt.Errorf("unknown charset: %s", charset)
		}
	}

	var dalyExchangeRates ValCurs
	err := d.Decode(&dalyExchangeRates)
	if err != nil {
		panic(err)
	}
	return &dalyExchangeRates
}

func getDalyExchangeRates(date string) *ValCurs {
	time.Sleep(50 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cbrURL+date, nil)
	if err != nil {
		panic(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	dalyExchangeRates := decodeWindows1251(resp.Body)
	return dalyExchangeRates
}

func stringToFloat64(v string) float64 {
	valueStr := strings.Replace(v, ",", ".", 1)
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		panic(err)
	}
	return value
}

func getTimeInterval() (time.Time, time.Time) {
	now := time.Now()
	start := now.AddDate(0, 0, -period+1)
	stop := now
	return start, stop
}

func setCurrencyDalyData(start time.Time, dataNinetyDays map[string]*Currency) map[string]*Currency {
	dalyExchangeRates := getDalyExchangeRates(start.Format("02/01/2006"))
	for _, currency := range dalyExchangeRates.Valute {
		nominal := stringToFloat64(currency.Nominal)
		value := stringToFloat64(currency.Value) / nominal
		curr, exist := dataNinetyDays[currency.CharCode]
		if !exist {
			newCurrency := Currency{Min: value, Max: value,
				MinDate: dalyExchangeRates.Date,
				MaxDate: dalyExchangeRates.Date,
				Sum:     value,
			}
			dataNinetyDays[currency.CharCode] = &newCurrency
			continue
		}
		if curr.Max < value {
			curr.Max = value
			curr.MaxDate = dalyExchangeRates.Date
		}
		if curr.Min > value {
			curr.Min = value
			curr.MinDate = dalyExchangeRates.Date
		}
		curr.Sum += value
	}
	return dataNinetyDays
}

func currencyInfo(start time.Time, stop time.Time) map[string]*Currency {
	dataNinetyDays := make(map[string]*Currency)

	for ; start.After(stop) == false; start = start.AddDate(0, 0, 1) {
		dataNinetyDays = setCurrencyDalyData(start, dataNinetyDays)
	}
	return dataNinetyDays
}

func main() {
	start, stop := getTimeInterval()
	startF, stopF := start.Format("02/01/2006"), stop.Format("02/01/2006")
	info := currencyInfo(start, stop)
	count := 1
	mLen := len(info)

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"#", "Currency", "Min (date)", "Max (date)", "AVG " + "(" + startF + " - " + stopF + ")"})
	for key, value := range info {
		if count != mLen {
			t.AppendRows([]table.Row{
				{count, key,
					fmt.Sprint(value.Min, " (", value.MinDate, ")"),
					fmt.Sprint(value.Max, " (", value.MaxDate, ")"),
					fmt.Sprint(value.Sum / float64(period))}})
			t.AppendSeparator()
		} else {
			t.AppendFooter(table.Row{
				count, key,
				fmt.Sprint(value.Min, " (", value.MinDate, ")"),
				fmt.Sprint(value.Max, " (", value.MaxDate, ")"),
				fmt.Sprint(value.Sum / float64(period)),
			})
		}
		count += 1
	}
	t.Render()
}
