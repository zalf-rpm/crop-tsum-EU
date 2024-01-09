package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// goal: generate maps of TSum for each crop, for each climate scenario
// to check if the required TSum can be reached for each crop, for each climate scenario
// it is assumed that the crop is sown at the earliest possible date and harvested at the latest possible date
// this is supposed to work for summer crops, but not for winter crops
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
	gridToRefFile := flag.String("grid_to_ref", "stu_eu_layer_grid.csv", "grid to reference mapping file")
	outputFolder := flag.String("output", "./output", "output folder")

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
	referenceToGridCode, gridCodeToReferences, err := readClimateRefData(*referenceFile)
	if err != nil {
		log.Fatal(err)
	}
	numberRef := len(referenceToGridCode)

	// read time range data from csv file
	timeRanges := readTimeRangeData(*sowingDateFile, *harvestDateFile, *sowingDefaultDOY, *harvestDefaultDOY, numberRef, *startYear, *endYear)

	// calculation result array
	calculationResult := make([]*CalculationResultRef, numberRef)

	for gridCode, refIds := range gridCodeToReferences {
		// add weather grid code to path
		weatherFileName := fmt.Sprintf(*pathToWeather, gridCode)
		//check if file exists
		if _, err := os.Stat(weatherFileName); os.IsNotExist(err) {
			// file does not exist
			log.Fatal(err)
		}

		// open weather file and calculate TSum for crop, for each reference
		calcResult, err := doCalculationPerWeatherFile(&crop, timeRanges, refIds, *startYear, *endYear, weatherFileName)
		if err != nil {
			log.Fatal(err)
		}
		// store calculation result
		for _, result := range calcResult {
			calculationResult[result.refId-1] = result
		}
	}
	// write calculation result to csv file and ascii grid
	err = writeCalculationResult(calculationResult, referenceToGridCode, *gridToRefFile, *startYear, *endYear, *outputFolder)
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
	refId            int
	Tsum             []float64 // TSum for each year
	frostDays        []float64 // number of frost days for each year
	TsumReached      []bool    // TSum reached maturity for each year
	WetHarvestYears  []bool    // years with wet harvest
	TsumReachedCount int       // number of years TSum reached maturity
	TsumAvg          float64   // average TSum for all years
	FrostOccurrence  int       // frost occurrence (number of years with frost)
	WetHarvest       int       // number of years with wet harvest
}

func doCalculationPerWeatherFile(crop *Crop, timeRanges []*TimeRange, refIds []int, startYear, endYear int, weatherFileName string) ([]*CalculationResultRef, error) {

	// calculation result array
	calculationResult := make([]*CalculationResultRef, len(refIds))
	refStages := make([]*refStage, len(refIds))
	harvestRain := make([]*harvestRainDays, len(refIds))
	for i, refId := range refIds {
		calculationResult[i] = &CalculationResultRef{
			refId:           refId,
			Tsum:            make([]float64, endYear-startYear+1),
			frostDays:       make([]float64, endYear-startYear+1),
			TsumReached:     make([]bool, endYear-startYear+1),
			WetHarvestYears: make([]bool, endYear-startYear+1),
		}
		refStages[i] = &refStage{
			refId:    refId,
			stageIdx: 0,
			Tsum:     0,
		}
		harvestRain[i] = newHarvestRainDays(refId)
	}
	// open weather file
	weatherFile, err := os.Open(weatherFileName)
	if err != nil {
		return nil, err
	}
	defer weatherFile.Close()
	scanner := bufio.NewScanner(weatherFile)
	headlines := 2
	idxTavg := -1
	idxTmin := -1
	idxDate := -1
	idxPrecip := -1
	currentYear := -1
	currentYearIdx := -1
	for scanner.Scan() {
		line := scanner.Text()
		// parse header line and get index for tavg, tmin and date
		if headlines > 0 {
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
				if field == "precip" {
					idxPrecip = idx
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
			return nil, err
		}
		// check if year is in range
		if year < startYear {
			continue
		}
		if year > endYear {
			break
		}
		// check if year has changed
		if year != currentYear {
			currentYear = year
			currentYearIdx++
			// reset stage index and TSum for each reference
			for _, rs := range refStages {
				rs.stageIdx = 0
				rs.Tsum = 0
			}
			// reset harvest date for each reference
			for _, hr := range harvestRain {
				hr.harvestDoy = -1
				hr.numWetHarvest = 0
			}
		}
		// get doy from date
		// convert date to DOY
		dateTime, err := time.Parse("2006-01-02", date)
		if err != nil {
			return nil, err
		}
		doy := dateTime.YearDay()

		// parse avgerage temperature
		tavg, err := strconv.ParseFloat(fields[idxTavg], 64)
		if err != nil {
			return nil, err
		}
		// parse minimum temperature
		tmin, err := strconv.ParseFloat(fields[idxTmin], 64)
		if err != nil {
			return nil, err
		}
		precip, err := strconv.ParseFloat(fields[idxPrecip], 64)
		if err != nil {
			return nil, err
		}
		// calculate TSum for each crop, for each reference
		for idx, refId := range refIds {

			// count wet harvest days before doy check if crop is in season
			// harvest may be after end of season
			calcHarRain := harvestRain[idx].countWetHarvestDays(doy, precip)
			if calcHarRain && harvestRain[idx].numWetHarvest > 0 {
				calculationResult[idx].WetHarvestYears[currentYearIdx] = true
			}

			// check if date is in vegetation period / time range
			if doy < timeRanges[year-startYear].StartDOY[refId-1] || doy > timeRanges[year-startYear].EndDOY[refId-1] {
				continue
			}

			// calculate TSum for each crop, for each reference
			tsum := calculateTSum(refStages[idx], crop, tavg)
			// calculate stage for crop
			calcStage(refStages[idx], crop, tsum)
			calculationResult[idx].Tsum[currentYearIdx] += tsum
			// set harvest date
			if harvestRain[idx].harvestDoy <= 0 && calculationResult[idx].Tsum[currentYearIdx] >= crop.TsumMaturity {
				harvestRain[idx].harvestDoy = doy
			}
			// calculate frost days
			if tmin < crop.FrostTreashold &&
				calculationResult[idx].Tsum[currentYearIdx] > 0 &&
				calculationResult[idx].Tsum[currentYearIdx] < crop.TsumMaturity {
				calculationResult[idx].frostDays[currentYearIdx]++
			}
		}
	}
	// for each reference
	// tsum reached maturity
	// avg tsum
	// frost occurrence
	for idx := range refIds {
		for yearIdx := 0; yearIdx < endYear-startYear+1; yearIdx++ {
			// tsum reached maturity
			if calculationResult[idx].Tsum[yearIdx] >= crop.TsumMaturity {
				calculationResult[idx].TsumReached[yearIdx] = true
			}
			// avg TSum
			calculationResult[idx].TsumAvg += calculationResult[idx].Tsum[yearIdx]
			// frost occurrence
			if calculationResult[idx].frostDays[yearIdx] > 0 {
				calculationResult[idx].FrostOccurrence++
			}
			// number of years TSum reached maturity
			if calculationResult[idx].TsumReached[yearIdx] {
				calculationResult[idx].TsumReachedCount++
			}
			// number of years with wet harvest
			if calculationResult[idx].WetHarvestYears[yearIdx] {
				calculationResult[idx].WetHarvest++
			}
		}
		// avg TSum
		calculationResult[idx].TsumAvg /= float64(endYear - startYear + 1)
	}
	return calculationResult, nil
}

type refStage struct {
	refId    int
	stageIdx int
	Tsum     float64
}

func calculateTSum(rs *refStage, c *Crop, tavg float64) float64 {
	// calculate temperature sum for crop
	temp := tavg - c.Stages[rs.stageIdx].BaseTemp
	if temp < 0 {
		temp = 0
	}
	return temp
}

func calcStage(rs *refStage, c *Crop, tsumDay float64) {
	rs.Tsum += tsumDay
	// calculate stage for crop
	if rs.stageIdx+1 == len(c.Stages) {
		return
	}
	if rs.Tsum >= c.Stages[rs.stageIdx].Tsum {
		rs.stageIdx++
		rs.Tsum = 0
	}
}

type harvestRainDays struct {
	refId          int
	harvestDoy     int
	precipPrevDays dataLastDays
	numWetHarvest  int
}

func newHarvestRainDays(refId int) *harvestRainDays {
	return &harvestRainDays{
		refId:          refId,
		harvestDoy:     -1,
		precipPrevDays: newDataLastDays(15),
		numWetHarvest:  0,
	}
}

func (hw *harvestRainDays) countWetHarvestDays(doy int, precip float64) (calculated bool) {

	calculated = false
	hw.precipPrevDays.addDay(precip)
	// has harvest date been reached and this doy is harvest + offs
	if hw.harvestDoy > 0 && doy == hw.harvestDoy+10 {
		wetDayCounter := 0
		twoDryDaysInRowDry := false
		rainData := hw.precipPrevDays.getData() // get last 15 days
		for i, x := range rainData {
			if i > 4 && x > 0 {
				wetDayCounter++
			}
			if i > 4 && x == 0 && rainData[i-1] == 0 {
				twoDryDaysInRowDry = true
			}
		}
		if wetDayCounter >= 5 && !twoDryDaysInRowDry {
			hw.numWetHarvest++
		}
		calculated = true
	}
	return calculated
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
			StartDOY: make([]int, size),
			EndDOY:   make([]int, size),
		}
		for j := 0; j < size; j++ {
			timeRanges[i].StartDOY[j] = sowingDateDefault
			timeRanges[i].EndDOY[j] = harvestDefault
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
func readDOY(filename string, startYear, endYear int, timeRanges []*TimeRange, isSow bool) error {

	// open csv file
	var reader io.Reader
	// check if file ends with .csv or .gz
	if strings.HasSuffix(filename, ".gz") {
		// open gzip file
		gzipFile, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer gzipFile.Close()

		gzipReader, err := gzip.NewReader(gzipFile)
		if err != nil {
			return err
		}
		defer gzipReader.Close()
		reader = gzipReader
	} else {
		// open csv file
		csvFile, err := os.Open(filename)
		if err != nil {
			return err
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
			return err
		}
		refIndex := refId - 1

		// parse DOY
		doy, err := strconv.Atoi(fields[1])
		if err != nil {
			return err
		}
		// get year
		date := fields[2]
		year, err := strconv.Atoi(date[0:4])
		if err != nil {
			return err
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
	return nil
}

// climate reference data
func readClimateRefData(filename string) (referenceToWeatherGridCode []string, GridCodeReferences map[string][]int, err error) {

	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	referenceToWeatherGridCode = make([]string, 0, defaultRefSize)
	GridCodeReferences = make(map[string][]int)

	skipHeader := true
	for scanner.Scan() {
		line := scanner.Text()
		// skip header line
		if skipHeader {
			skipHeader = false
			continue
		}
		// split line
		fields := strings.Split(line, ",")
		// parse refId
		refId, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, nil, err
		}
		// climate reference
		weatherGridCode := fields[1]
		referenceToWeatherGridCode = append(referenceToWeatherGridCode, weatherGridCode)
		// climate reference to refId
		GridCodeReferences[weatherGridCode] = append(GridCodeReferences[weatherGridCode], refId)
	}
	return referenceToWeatherGridCode, GridCodeReferences, nil
}

// write calculation result to csv file and ascii grid
func writeCalculationResult(calculationResult []*CalculationResultRef, referenceToClim []string, gridToRefFile string, startYear, endYear int, outpuFolder string) error {
	// write calculation result to csv file
	csvFileName := filepath.Join(outpuFolder, fmt.Sprintf("cal_res_ref_%d-%d.csv", startYear, endYear))
	csvFile, err := createGzFileWriter(csvFileName)
	if err != nil {
		return err
	}
	defer csvFile.Close()
	// write header line
	_, err = csvFile.Write("refId,climate,year,Tsum,frost_days,Tsum_reached,Wet_Harvest\n")
	if err != nil {
		return err
	}

	for _, result := range calculationResult {
		for yearIdx := 0; yearIdx < endYear-startYear+1; yearIdx++ {
			_, err = csvFile.Write(fmt.Sprintf("%d,%s,%d,%f,%f,%t,%t\n", result.refId, referenceToClim[result.refId-1], startYear+yearIdx, result.Tsum[yearIdx], result.frostDays[yearIdx], result.TsumReached[yearIdx], result.WetHarvestYears[yearIdx]))
			if err != nil {
				return err
			}
		}
	}
	// load grid to reference mapping
	rowExt, colExt, gridToRef, err := GetGridLookup(gridToRefFile)
	if err != nil {
		return err
	}

	// --------------------
	writeGrid := func(ascFileNameTempl string, outType outputType) error {
		ascFileName := filepath.Join(outpuFolder, fmt.Sprintf(ascFileNameTempl, startYear, endYear))
		fout, err := createGridFile(ascFileName, colExt, rowExt)
		if err != nil {
			return err
		}
		defer fout.Close()
		err = writeRows(fout, rowExt, colExt, calculationResult, outType, gridToRef)
		if err != nil {
			return err
		}
		return nil
	}

	// write calculation result to ascii grids
	// TsumAvg
	err = writeGrid("TsumAvg_%d-%d.asc", TSumAvg)
	if err != nil {
		return err
	}
	// TsumReached
	err = writeGrid("TsumReached_%d-%d.asc", TSumReached)
	if err != nil {
		return err
	}
	// FrostOccurrence
	err = writeGrid("FrostOccurrence_%d-%d.asc", FrostOccurrence)
	if err != nil {
		return err
	}
	// WetHarvest
	err = writeGrid("WetHarvest_%d-%d.asc", WetHarvest)
	if err != nil {
		return err
	}
	return nil
}

func createGridFile(name string, nCol, nRow int) (*Fout, error) {
	cornerX := 0.0
	cornery := 0.0
	novalue := -9999
	cellsize := 1.0

	fout, err := createGzFileWriter(name)
	if err != nil {
		return nil, err
	}

	fout.Write(fmt.Sprintf("ncols %d\n", nCol))
	fout.Write(fmt.Sprintf("nrows %d\n", nRow))
	fout.Write(fmt.Sprintf("xllcorner     %f\n", cornerX))
	fout.Write(fmt.Sprintf("yllcorner     %f\n", cornery))
	fout.Write(fmt.Sprintf("cellsize      %f\n", cellsize))
	fout.Write(fmt.Sprintf("NODATA_value  %d\n", novalue))

	return fout, nil
}

// emum for output type
type outputType int

const (
	// output type for TSumAvg
	TSumAvg outputType = iota
	// output type for TSumReached
	TSumReached
	// output type for FrostOccurrence
	FrostOccurrence
	// output type for WetHarvest
	WetHarvest
)

func writeRows(fout *Fout, extRow, extCol int, calcResults []*CalculationResultRef, outType outputType, gridSourceLookup [][]int) error {
	size := len(calcResults)
	for row := 0; row < extRow; row++ {

		for col := 0; col < extCol; col++ {
			refID := gridSourceLookup[row][col]
			var err error
			if refID >= 0 && refID < size {
				if outType == TSumAvg {
					_, err = fout.Write(strconv.Itoa(int(math.Round(calcResults[refID-1].TsumAvg))))
				} else if outType == TSumReached {
					_, err = fout.Write(strconv.Itoa(calcResults[refID-1].TsumReachedCount))
				} else if outType == FrostOccurrence {
					_, err = fout.Write(strconv.Itoa(calcResults[refID-1].FrostOccurrence))
				} else if outType == WetHarvest {
					_, err = fout.Write(strconv.Itoa(calcResults[refID-1].WetHarvest))
				} else {
					_, err = fout.Write("-9999")
				}
				if err != nil {
					return err
				}
				_, err = fout.Write(" ")
			} else {
				_, err = fout.Write("-9999 ")
			}
			if err != nil {
				return err
			}
		}
		_, err := fout.Write("\n")
		if err != nil {
			return err
		}
	}
	return nil
}

// Fout combined file writer
type Fout struct {
	file    *os.File
	gfile   *gzip.Writer
	fwriter *bufio.Writer
}

// create gz file writer
func createGzFileWriter(name string) (*Fout, error) {
	// extract folder name
	filepath.Dir(name)
	folder := filepath.Dir(name)
	if folder == "" {
		folder = "."
	}
	// create folder if not exists
	if _, err := os.Stat(folder); os.IsNotExist(err) {
		// folder does not exist
		err = os.Mkdir(folder, 0755)
		if err != nil {
			return nil, err
		}
	}
	file, err := os.OpenFile(name+".gz", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	gfile := gzip.NewWriter(file)
	fwriter := bufio.NewWriter(gfile)
	return &Fout{file, gfile, fwriter}, nil
}

// Write string to zip file
func (f Fout) Write(s string) (count int, err error) {
	count, err = f.fwriter.WriteString(s)
	return count, err
}

// Close file writer
func (f Fout) Close() error {
	err := f.fwriter.Flush()
	if err != nil {
		return err
	}
	// Close the gzip first.
	err = f.gfile.Close()
	if err != nil {
		return err
	}
	err = f.file.Close()
	return err
}

// Get GridLookup
func GetGridLookup(gridsource string) (rowExt int, colExt int, lookupGrid [][]int, err error) {
	type GridCoord struct {
		row int
		col int
	}
	colExt = 0
	rowExt = 0
	lookup := make(map[int64][]GridCoord)

	sourcefile, err := os.Open(gridsource)
	if err != nil {
		return 0, 0, nil, err
	}
	defer sourcefile.Close()
	firstLine := true
	colID := -1
	rowID := -1
	refID := -1
	scanner := bufio.NewScanner(sourcefile)
	for scanner.Scan() {
		line := scanner.Text()
		tokens := strings.Split(line, ",")
		if firstLine {
			firstLine = false
			for index, token := range tokens {
				if token == "Column_" {
					colID = index
				}
				if token == "Row" {
					rowID = index
				}
				if token == "soil_ref" {
					refID = index
				}
			}
		} else {
			col, _ := strconv.ParseInt(tokens[colID], 10, 64)
			row, _ := strconv.ParseInt(tokens[rowID], 10, 64)
			ref, _ := strconv.ParseInt(tokens[refID], 10, 64)
			if int(col) > colExt {
				colExt = int(col)
			}
			if int(row) > rowExt {
				rowExt = int(row)
			}
			if _, ok := lookup[ref]; !ok {
				lookup[ref] = make([]GridCoord, 0, 1)
			}
			lookup[ref] = append(lookup[ref], GridCoord{int(row), int(col)})
		}
	}
	lookupGrid = newGrid(rowExt, colExt, -1)
	for ref, coord := range lookup {
		for _, rowCol := range coord {
			lookupGrid[rowCol.row-1][rowCol.col-1] = int(ref)
		}
	}

	return rowExt, colExt, lookupGrid, nil
}

// create new grid with default values
func newGrid(extRow, extCol, defaultVal int) [][]int {
	grid := make([][]int, extRow)
	for r := 0; r < extRow; r++ {
		grid[r] = make([]int, extCol)
		for c := 0; c < extCol; c++ {
			grid[r][c] = defaultVal
		}
	}
	return grid
}
