package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// goal: generate maps of TSum for each crop, for each climate scenario
// to check if the required TSum can be reached for each crop, for each climate scenario
// read weather files from climate scenarios
// calculate TSum for each crop, with a given start date and end date
// calculate maps for risks of frost and rain in the harvest period
// required input of crop data:
// - start date (DOY)
// - end date (DOY)
// - TSum required to reach maturity
// - base temperature
//	TBD: base temperature depends on development stage and varies quite a lot between development stages
// better use base temperatures for each development stage
// and have a TSum for each base temperature or stage
// stages, Tbase, Tsum can be taken from the literature or from the crop model MONICA or HERMES

// required input of weather data:
// - temperature (daily average)
// - precipitation (daily total)
// climate scenarios:
// - historical
// - RCP 4.5
// - RCP 8.5
// time period:
// - 1980-2010
// - 2040-2070
// - 2070-2100
// output:
// - maps of TSum for each crop, for each climate scenario, for each time period( average of 30 years)
// - maps of risks for each crop, for each climate scenario:
//   - frost in growing period
//   - rain in the harvest period

const defaultRefSize = 99367 // number of climate references from soybeanEU project

func main() {

	// parse crop from command line
	cropFileName := flag.String("crop", "soybean.yml", "crop file name")
	createCropFile := flag.Bool("create_crop", false, "create crop file")
	sowingDateFile := flag.String("sowing", "sowing_date.csv", "sowing dates file name")
	sowingDefaultDOY := flag.Int("sowing_default", 150, "default sowing date (DOY)")
	harvestDateFile := flag.String("harvest", "", "harvest dates file name")
	harvestDefaultDOY := flag.Int("harvest_default", 300, "default harvest date (DOY)")
	startYear := flag.Int("start_year", 1980, "start year")
	endYear := flag.Int("end_year", 2010, "end year")
	pathToWeather := flag.String("weather", "weather", "path to weather files")

	referenceFile := flag.String("reference", "stu_eu_layer_ref.csv", "reference file climate sowing date mapping")

	flag.Parse()

	if *createCropFile {
		// create crop file
		// marshal yml file
		// write yml file
		err := generateCropFile(*cropFileName)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	// read crop data from yml file
	crop, err := readCropData(*cropFileName)
	if err != nil {
		log.Fatal(err)
	}

	// read reference data from csv file
	referenceToClim, climToReference := readClimateRefData(*referenceFile)
	numberRef := len(referenceToClim)

	// read time range data from csv file
	timeRanges := readTimeRangeData(*sowingDateFile, *harvestDateFile, *sowingDefaultDOY, *harvestDefaultDOY, numberRef, *startYear, *endYear)

	// calculation result array
	calculationResult := make([]*CalculationResultRef, numberRef)

	for climateId, refIds := range climToReference {
		// get climate scenario
		// get weather data
		weatherFileName := *pathToWeather + "/" + climateId + ".csv"
		//check if file exists
		if _, err := os.Stat(weatherFileName); os.IsNotExist(err) {
			// file does not exist
			weatherFileName = *pathToWeather + "/" + climateId + ".csv.gz"
			if _, err := os.Stat(weatherFileName); os.IsNotExist(err) {
				// file does not exist
				log.Fatal(err)
			}
		}

		// open weather file and calculate TSum for crop, for each reference
		calcResult := doCalculationPerWeatherFile(&crop, timeRanges, refIds, *startYear, *endYear, weatherFileName)

		// store calculation result
		for _, result := range calcResult {
			calculationResult[result.refId] = result
		}
	}
	// write calculation result to csv file and ascii grid
	err = writeCalculationResult(calculationResult, referenceToClim, *startYear, *endYear)
	if err != nil {
		log.Fatal(err)
	}
}

func generateCropFile(cropFileName string) error {
	crop := Crop{
		Name:         "soybean_0", // from crop model monica soybeanEU
		TsumMaturity: 2235,
		Stages: []Stage{
			{
				Name:     "germination",
				Tsum:     167,
				BaseTemp: 8,
			},
			{
				Name:     "flowering",
				Tsum:     1048,
				BaseTemp: 6,
			},
			{
				Name:     "maturity",
				Tsum:     1058,
				BaseTemp: 6,
			},
		},
		FrostTreashold: 5, // temperature below which cold damage occurs
	}

	data, err := yaml.Marshal(&crop)
	if err != nil {
		return err
	}

	err = os.WriteFile(cropFileName, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

type CalculationResultRef struct {
	refId     int
	Tsum      float64
	frostDays float64
}

func doCalculationPerWeatherFile(crop *Crop, timeRanges []*TimeRange, refIds []int, startYear, endYear int, weatherFileName string) []*CalculationResultRef {

	// calculation result array
	calculationResult := make([]*CalculationResultRef, len(refIds))
	refStages := make([]*refStage, len(refIds))
	for i, refId := range refIds {
		calculationResult[i] = &CalculationResultRef{
			refId:     refId,
			Tsum:      0,
			frostDays: 0,
		}
		refStages[i] = &refStage{
			refId:    refId,
			stageIdx: 0,
			Tsum:     0,
		}
	}
	// open weather file
	weatherFile, err := os.Open(weatherFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer weatherFile.Close()
	scanner := bufio.NewScanner(weatherFile)
	headlines := 1
	idxTavg := -1
	idxTmin := -1
	idxDate := -1
	for scanner.Scan() {
		line := scanner.Text()
		// parse header line and get index for tavg, tmin and date
		if headlines == 1 {
			fields := strings.Split(line, ",")
			for idx, field := range fields {
				if field == "tavg" {
					idxTavg = idx
				}
				if field == "tmin" {
					idxTmin = idx
				}
				if field == "iso-date" || field == "date" {
					idxDate = idx
				}
			}
			headlines--
			continue
		}
		// split line
		fields := strings.Split(line, ",")
		// parse date
		date := fields[idxDate]
		year, err := strconv.Atoi(date[0:4])
		if err != nil {
			log.Fatal(err)
		}
		// check if year is in range
		if year < startYear || year > endYear {
			continue
		}
		// get doy from date
		// convert date to DOY
		dateTime, err := time.Parse("2006-01-02", date)
		if err != nil {
			log.Fatal(err)
		}
		doy := dateTime.YearDay()

		// parse avgerage temperature
		tavg, err := strconv.ParseFloat(fields[idxTavg], 64)
		if err != nil {
			log.Fatal(err)
		}
		// parse minimum temperature
		tmin, err := strconv.ParseFloat(fields[idxTmin], 64)
		if err != nil {
			log.Fatal(err)
		}
		// calculate TSum for each crop, for each reference
		for idx, refId := range refIds {

			// check if date is in vegetation period / time range
			if doy < timeRanges[year-startYear].StartDOY[refId-1] || doy > timeRanges[year-startYear].EndDOY[refId-1] {
				continue
			}

			// calculate TSum for each crop, for each reference
			tsum := calculateTSum(refStages[idx], crop, tavg)
			// calculate stage for crop
			calcStage(refStages[idx], crop, tsum)
			calculationResult[idx].Tsum += tsum
			// calculate frost days
			if tmin < crop.FrostTreashold {
				calculationResult[idx].frostDays++
			}

		}
	}
	return calculationResult
}

type refStage struct {
	refId    int
	stageIdx int
	Tsum     float64
}

func calculateTSum(rs *refStage, c *Crop, tavg float64) (tsum float64) {
	// calculate temperature sum for crop
	temp := tavg - c.Stages[rs.stageIdx].BaseTemp
	if temp < 0 {
		temp = 0
	}
	return tsum
}

func calcStage(rs *refStage, c *Crop, tsumDay float64) {
	rs.Tsum += tsumDay
	// calculate stage for crop
	if rs.stageIdx >= len(c.Stages) {
		return
	}
	if rs.Tsum >= c.Stages[rs.stageIdx].Tsum {
		rs.stageIdx++
		rs.Tsum = 0
	}
}

type TimeRange struct {
	StartDOY []int // start date (DOY) - earliest possible sowing date
	EndDOY   []int // end date (DOY) - latest possible harvest date
}

// crop data
type Crop struct {
	Name string // crop name

	TsumMaturity   float64 // TSum required to reach maturity
	Stages         []Stage
	FrostTreashold float64 // temperature below which frost occurs
}

type Stage struct {
	Name     string  // optional
	Tsum     float64 // TSum required to reach this stage
	BaseTemp float64 // base temperature for this stage
}

// read crop data from yml file
func readCropData(filename string) (crop Crop, err error) {

	crop = Crop{
		Name:         "",
		TsumMaturity: 0,
		Stages:       nil,
	}
	// read crop data from yml file
	data, err := os.ReadFile(filename)
	if err != nil {
		return crop, err
	}
	// unmarshal yml file
	err = yaml.Unmarshal(data, &crop)
	if err != nil {
		return crop, err
	}
	return crop, nil
}

// read time range data from csv file
func readTimeRangeData(sowingDateFile, harvestDateFile string, sowingDateDefault, harvestDefault, size, startYear, endYear int) (timeRanges []*TimeRange) {

	numberYears := endYear - startYear + 1
	// create time range data
	timeRanges = make([]*TimeRange, numberYears)

	// set default time range
	for i := 0; i < numberYears; i++ {
		timeRanges[i] = &TimeRange{
			StartDOY: make([]int, sowingDateDefault, size),
			EndDOY:   make([]int, harvestDefault, size),
		}
	}

	// read time range data from csv file
	if sowingDateFile != "" {
		// read sowing date data from csv file
		readDOY(sowingDateFile, startYear, endYear, timeRanges, true)
	}
	if harvestDateFile != "" {
		// read harvest date data from csv file
		readDOY(harvestDateFile, startYear, endYear, timeRanges, false)
	}

	return timeRanges
}

// read DOY from csv file
func readDOY(filename string, startYear, endYear int, timeRanges []*TimeRange, isSow bool) {

	// open csv file
	var reader io.Reader
	// check if file ends with .csv or .gz
	if strings.HasSuffix(filename, ".gz") {
		// open gzip file
		gzipFile, err := os.Open(filename)
		if err != nil {
			log.Fatal(err)
		}
		defer gzipFile.Close()

		gzipReader, err := gzip.NewReader(gzipFile)
		if err != nil {
			log.Fatal(err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	} else {
		// open csv file
		csvFile, err := os.Open(filename)
		if err != nil {
			log.Fatal(err)
		}
		defer csvFile.Close()
		reader = csvFile
	}

	// read csv file
	//refId,DOY,Date
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		// skip header line
		if strings.HasPrefix(line, "refId") {
			continue
		}
		// split line
		fields := strings.Split(line, ",")
		// parse refId
		refId, err := strconv.Atoi(fields[0])
		if err != nil {
			log.Fatal(err)
		}
		refIndex := refId - 1

		// parse DOY
		doy, err := strconv.Atoi(fields[1])
		if err != nil {
			log.Fatal(err)
		}
		// get year
		date := fields[2]
		year, err := strconv.Atoi(date[0:4])
		if err != nil {
			log.Fatal(err)
		}
		// check if year is in range
		if year < startYear || year > endYear {
			continue
		}
		// get index for year
		yearIndex := year - startYear

		if isSow {
			timeRanges[yearIndex].StartDOY[refIndex] = doy
		} else {
			timeRanges[yearIndex].EndDOY[refIndex] = doy
		}
	}
}

// climate reference data
func readClimateRefData(filename string) (referenceToClim []string, climToReference map[string][]int) {

	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	referenceToClim = make([]string, 0, defaultRefSize)
	climToReference = make(map[string][]int)

	for scanner.Scan() {
		line := scanner.Text()
		// skip header line
		if strings.HasPrefix(line, "refId") {
			continue
		}
		// split line
		fields := strings.Split(line, ",")
		// parse refId
		refId, err := strconv.Atoi(fields[0])
		if err != nil {
			log.Fatal(err)
		}
		// climate reference
		clim := fields[1]
		referenceToClim[refId-1] = clim
		// climate reference to refId
		climToReference[clim] = append(climToReference[clim], refId)
	}
	return referenceToClim, climToReference
}

// write calculation result to csv file and ascii grid
func writeCalculationResult(calculationResult []*CalculationResultRef, referenceToClim []string, startYear, endYear int) error {
	// TODO
	return nil
}
