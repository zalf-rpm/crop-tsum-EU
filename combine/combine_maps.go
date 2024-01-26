package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

// takes ascii grid and combines it within one map
// read config file for ascii grid names, path and output

func main() {
	writeConf := flag.Bool("write-config", false, "write config file")
	confPath := flag.String("config", "config.yml", "path to config file")
	crop := flag.String("crop", "chickpea", "crop name")
	cropPath := flag.String("crop-path", "crop", "crop path")

	flag.Parse()

	// print current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Current working directory:", cwd)

	if *writeConf {
		writeConfig(*confPath)
		return
	}
	// read config file
	configs := readConfig(*confPath)

	// read ascii grids
	for _, config := range configs {
		// read ascii grids
		asciiGrids45 := readAsciiGrids(config.AsciiGrids45, *cropPath)
		asciiGrids85 := readAsciiGrids(config.AsciiGrids85, *cropPath)
		asciiGridHistorical := readAsciiGrids(config.AsciiGridHistorical, *cropPath)

		// combine ascii grids
		combinedAsciiGridHistorical := combineAsciiGrids(asciiGridHistorical, config.CombineMode, config.Threshold, config.DefaultMin)
		combinedGrid45 := combineAsciiGrids(asciiGrids45, config.CombineMode, config.Threshold, config.DefaultMin)
		combinedGrid85 := combineAsciiGrids(asciiGrids85, config.CombineMode, config.Threshold, config.DefaultMin)

		// combine historical and future grids meta data
		combinedGridMeta := combineHistoricalFutureMeta(combinedAsciiGridHistorical.Meta, combinedGrid45.Meta, combinedGrid85.Meta)

		// write combined grid
		writeAsciiGrid(combinedGrid45, config.OutPath, config.OutputGridTempl, "45", *crop)
		writeAsciiGrid(combinedGrid85, config.OutPath, config.OutputGridTempl, "85", *crop)
		writeAsciiGrid(combinedAsciiGridHistorical, config.OutPath, config.OutputGridTempl, "historical", *crop)

		// write metadata
		writeMeta(combinedGridMeta, config.OutPath, config.OutputGridTempl, "historical", *crop, "(a)")
		writeMeta(combinedGridMeta, config.OutPath, config.OutputGridTempl, "45", *crop, "(b)")
		writeMeta(combinedGridMeta, config.OutPath, config.OutputGridTempl, "85", *crop, "(c)")
	}
}

type AsciiGrid struct {
	// grid data
	Data [][]float64
	// grid meta data
	Meta *AsciiGridMeta
}
type AsciiGridMeta struct {
	// number of columns
	NCols int
	// number of rows
	NRows int
	// xll corner
	XllCorner float64
	// yll corner
	YllCorner float64
	// cell size
	CellSize float64
	// no data value
	NoDataValue float64
	// min value in grid
	Min float64
	// max value in grid
	Max float64
}

func readAsciiGrids(paths []string, croppath string) []*AsciiGrid {
	// read ascii grids
	asciiGrids := make([]*AsciiGrid, len(paths))
	for i, path := range paths {
		asciiGrids[i] = readAsciiGrid(fmt.Sprintf(path, croppath))
	}
	return asciiGrids
}

// read ascii grid
func readAsciiGrid(path string) *AsciiGrid {
	// open file
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// read gzip file
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		log.Fatal(err)
	}
	defer gzipReader.Close()

	// read ascii grid header
	scan := bufio.NewScanner(gzipReader)

	asciiGrid := &AsciiGrid{
		Data: [][]float64{},
		Meta: &AsciiGridMeta{
			NCols:       0,
			NRows:       0,
			XllCorner:   0,
			YllCorner:   0,
			CellSize:    0,
			NoDataValue: 0,
			Min:         0,
			Max:         0,
		},
	}

	// first 6 lines are header
	index := -1
	for scan.Scan() {
		index++
		if index < 6 {
			// read header
			header := scan.Text()
			if strings.HasPrefix(header, "ncols") {
				// remove prefix
				header = strings.TrimPrefix(header, "ncols")
				// remove spaces
				header = strings.TrimSpace(header)
				// convert to int
				asciiGrid.Meta.NCols, err = strconv.Atoi(header)
			}
			if strings.HasPrefix(header, "nrows") {
				// remove prefix
				header = strings.TrimPrefix(header, "nrows")
				// remove spaces
				header = strings.TrimSpace(header)
				// convert to int
				asciiGrid.Meta.NRows, err = strconv.Atoi(header)
			}
			if strings.HasPrefix(header, "xllcorner") {
				// remove prefix
				header = strings.TrimPrefix(header, "xllcorner")
				// remove spaces
				header = strings.TrimSpace(header)
				// convert to float
				asciiGrid.Meta.XllCorner, err = strconv.ParseFloat(header, 64)

			}
			if strings.HasPrefix(header, "yllcorner") {
				// remove prefix
				header = strings.TrimPrefix(header, "yllcorner")
				// remove spaces
				header = strings.TrimSpace(header)
				// convert to float
				asciiGrid.Meta.YllCorner, err = strconv.ParseFloat(header, 64)

			}
			if strings.HasPrefix(header, "cellsize") {
				// remove prefix
				header = strings.TrimPrefix(header, "cellsize")
				// remove spaces
				header = strings.TrimSpace(header)
				// convert to float
				asciiGrid.Meta.CellSize, err = strconv.ParseFloat(header, 64)

			}
			if strings.HasPrefix(header, "NODATA_value") {
				// remove prefix
				header = strings.TrimPrefix(header, "NODATA_value")
				// remove spaces
				header = strings.TrimSpace(header)
				// convert to float
				asciiGrid.Meta.NoDataValue, err = strconv.ParseFloat(header, 64)
				asciiGrid.Meta.Min = asciiGrid.Meta.NoDataValue
				asciiGrid.Meta.Max = asciiGrid.Meta.NoDataValue
			}
			if err != nil {
				log.Fatal(err)
			}
		} else {
			if index == 6 {
				// set size of data
				asciiGrid.Data = make([][]float64, asciiGrid.Meta.NRows)
				for i := range asciiGrid.Data {
					asciiGrid.Data[i] = make([]float64, asciiGrid.Meta.NCols)
				}
			}
			// read data
			data := scan.Text()
			// split data
			dataSplit := strings.Split(data, " ")
			// convert to float
			for i, d := range dataSplit {
				if d == "" {
					continue
				}
				asciiGrid.Data[index-6][i], err = strconv.ParseFloat(d, 64)
				if err != nil {
					log.Fatal(err)
				}
				// set min and max
				asciiGrid.min(asciiGrid.Data[index-6][i])
				asciiGrid.max(asciiGrid.Data[index-6][i])
			}
		}
	}

	return asciiGrid
}
func (as *AsciiGrid) min(newVal float64) {
	if newVal == as.Meta.NoDataValue {
		return
	}
	if as.Meta.Min == as.Meta.NoDataValue {
		as.Meta.Min = newVal
	} else if newVal < as.Meta.Min {
		as.Meta.Min = newVal
	}
}
func (as *AsciiGrid) max(newVal float64) {
	if newVal == as.Meta.NoDataValue {
		return
	}
	if as.Meta.Max == as.Meta.NoDataValue {
		as.Meta.Max = newVal
	} else if newVal > as.Meta.Max {
		as.Meta.Max = newVal
	}
}

// combine modes (avg, avgthreshold)

type CombineMode int

const (
	CMAvg CombineMode = iota
	CMAvgThreshold
	CMPairsWithThreshold // combine pairs with threshold, the even grid index is the base, the odd grid is the threshold grid
)

// combine ascii grids
func combineAsciiGrids(asciiGrids []*AsciiGrid, mode CombineMode, threshold, defaultMin float64) *AsciiGrid {

	combineMode := mode
	if combineMode == CMPairsWithThreshold && len(asciiGrids)%2 != 0 {
		log.Fatal("number of ascii grids must be even")
	}
	gridsToCombine := asciiGrids
	if combineMode == CMPairsWithThreshold && len(asciiGrids) > 2 {
		combindedPairs := make([]*AsciiGrid, 0, len(asciiGrids)/2)
		// combine pairs
		for i := 0; i < len(asciiGrids); i += 2 {
			combinded2Grids := combineAsciiGrids([]*AsciiGrid{asciiGrids[i], asciiGrids[i+1]}, combineMode, threshold, defaultMin)
			combindedPairs = append(combindedPairs, combinded2Grids)
		}
		gridsToCombine = combindedPairs
		combineMode = CMAvg
	}

	// create ascii grid for combined grid
	combinedGrid := &AsciiGrid{
		Data: make([][]float64, asciiGrids[0].Meta.NRows),
		Meta: &AsciiGridMeta{
			NCols:       asciiGrids[0].Meta.NCols,
			NRows:       asciiGrids[0].Meta.NRows,
			XllCorner:   asciiGrids[0].Meta.XllCorner,
			YllCorner:   asciiGrids[0].Meta.YllCorner,
			CellSize:    asciiGrids[0].Meta.CellSize,
			NoDataValue: asciiGrids[0].Meta.NoDataValue,
			Min:         asciiGrids[0].Meta.Min,
			Max:         asciiGrids[0].Meta.Max,
		},
	}
	// init grid data
	combinedGrid.Data = make([][]float64, combinedGrid.Meta.NRows)
	for i := range combinedGrid.Data {
		combinedGrid.Data[i] = make([]float64, combinedGrid.Meta.NCols)
	}

	// combine grids
	for i := range gridsToCombine {
		for j := range gridsToCombine[i].Data {
			for k := range gridsToCombine[i].Data[j] {
				// check if value is no data value
				if gridsToCombine[i].Data[j][k] == gridsToCombine[i].Meta.NoDataValue || combinedGrid.Data[j][k] == combinedGrid.Meta.NoDataValue {
					combinedGrid.Data[j][k] = combinedGrid.Meta.NoDataValue
				} else {
					// combine values
					switch combineMode {
					case CMAvg:
						combinedGrid.Data[j][k] += gridsToCombine[i].Data[j][k]
					case CMAvgThreshold:
						combinedGrid.Data[j][k] += gridsToCombine[i].Data[j][k]
					case CMPairsWithThreshold:
						thresholdGridValue := gridsToCombine[i+1].Data[j][k]
						if thresholdGridValue < threshold {
							combinedGrid.Data[j][k] = gridsToCombine[i].Data[j][k]
						} else {
							combinedGrid.Data[j][k] = defaultMin
						}
					}
				}
			}
		}
		if CMPairsWithThreshold == combineMode {
			// we can only combine pairs, so we can break here
			break
		}
	}
	for j := range combinedGrid.Data {
		for k := range combinedGrid.Data[j] {
			// check if value is no data value
			if combinedGrid.Data[j][k] == combinedGrid.Meta.NoDataValue {
				continue
			}
			// combine values
			switch combineMode {
			case CMAvg:
				combinedGrid.Data[j][k] /= float64(len(gridsToCombine))
				// set min and max
				combinedGrid.min(combinedGrid.Data[j][k])
				combinedGrid.max(combinedGrid.Data[j][k])
			case CMAvgThreshold:
				combinedGrid.Data[j][k] /= float64(len(gridsToCombine))
				// check if value is above threshold
				if combinedGrid.Data[j][k] < threshold {
					combinedGrid.Data[j][k] = 0
				}
				// set min and max
				combinedGrid.min(combinedGrid.Data[j][k])
				combinedGrid.max(combinedGrid.Data[j][k])
			}
		}
	}

	return combinedGrid
}

// combine historical and future grids meta data
func combineHistoricalFutureMeta(historicalMeta *AsciiGridMeta, futureMeta45 *AsciiGridMeta, futureMeta85 *AsciiGridMeta) *AsciiGridMeta {
	// combine meta data
	combinedMeta := &AsciiGridMeta{
		NCols:       historicalMeta.NCols,
		NRows:       historicalMeta.NRows,
		XllCorner:   historicalMeta.XllCorner,
		YllCorner:   historicalMeta.YllCorner,
		CellSize:    historicalMeta.CellSize,
		NoDataValue: historicalMeta.NoDataValue,
		Min:         historicalMeta.Min,
		Max:         historicalMeta.Max,
	}
	// get min and max
	combinedMeta.Max = math.Max(historicalMeta.Max, math.Max(futureMeta45.Max, futureMeta85.Max))
	combinedMeta.Min = math.Min(historicalMeta.Min, math.Min(futureMeta45.Min, futureMeta85.Min))

	return combinedMeta
}

type Config struct {
	// paths to ascii grids for 4.5
	AsciiGrids45 []string
	// paths to ascii grids for 8.5
	AsciiGrids85 []string
	// path to historical ascii grid
	AsciiGridHistorical []string

	// output path
	OutPath         string
	OutputGridTempl string

	CombineMode CombineMode
	Threshold   float64
	DefaultMin  float64
}

// write default config file
func writeConfig(confPath string) {

	// default configs
	config := map[string]Config{
		"config1": {
			AsciiGrids45:        []string{"path/to/ascii/%s/grid1", "path/to/ascii/%s/grid2"},
			AsciiGrids85:        []string{"path/to/ascii/%s/grid1", "path/to/ascii/%s/grid2"},
			AsciiGridHistorical: []string{"path/to/ascii/%s/grid_historical"},
			OutPath:             "path/to/output",
			OutputGridTempl:     "config1_%s_%s.asc",
			CombineMode:         CMAvg,
			Threshold:           -1,
			DefaultMin:          0,
		},
		"config2": {
			AsciiGrids45:        []string{"path/to/ascii/%s/grid1", "path/to/ascii/%s/grid2"},
			AsciiGrids85:        []string{"path/to/ascii/%s/grid1", "path/to/ascii/%s/grid2"},
			AsciiGridHistorical: []string{"path/to/ascii/%s/grid_historical"},
			OutPath:             "path/to/output",
			OutputGridTempl:     "config2_%s_%s.asc",
			CombineMode:         CMAvg,
			Threshold:           -1,
			DefaultMin:          0,
		},
	}

	// write config file yml
	file, err := os.Create(confPath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	// marshal config to yaml
	d, err := yaml.Marshal(&config)
	if err != nil {
		log.Fatal(err)
	}
	// write yaml to file
	_, err = file.Write(d)
	if err != nil {
		log.Fatal(err)
	}

}

func readConfig(confPath string) map[string]Config {
	// read config file
	config := make(map[string]Config)
	data, err := os.ReadFile(confPath)
	if err != nil {
		log.Fatal(err)
	}

	// unmarshal config to yaml
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatal(err)
	}
	return config
}

// write ascii grid
func writeAsciiGrid(asciiGrid *AsciiGrid, outPath, outTempl, name, crop string) {
	// create output file
	fout, err := createGridFile(filepath.Join(outPath, fmt.Sprintf(outTempl, crop, name)), asciiGrid.Meta)
	if err != nil {
		log.Fatal(err)
	}
	// write data
	for i := range asciiGrid.Data {
		for j := range asciiGrid.Data[i] {
			fout.Write(fmt.Sprintf("%f ", asciiGrid.Data[i][j]))
		}
		fout.Write("\n")
	}
	fout.Close()
}

// write meta data
func writeMeta(asciiGridMeta *AsciiGridMeta, outPath, outTempl, name, crop, title string) {
	// create output filename
	outname := filepath.Join(outPath, fmt.Sprintf(outTempl, crop, name))

	writeMetaFile(
		outname,                   // path+name to grid file
		title,                     // title
		"year",                    // labeltext
		"viridis",                 // colormap
		"",                        // colorlisttype
		nil,                       // colorlist []string
		nil,                       // cbarLabel []string
		nil,                       // ticklist []float64
		1,                         // factor
		asciiGridMeta.Max,         // maxValue
		asciiGridMeta.Min,         // minValue
		"lightgrey",               // minColor
		asciiGridMeta.NoDataValue) // nodata
}

// write meta data
func writeMetaFile(gridFilePath, title, labeltext, colormap, colorlistType string, colorlist []string, cbarLabel []string, ticklist []float64, factor float64, maxValue, minValue float64, minColor string, nodata float64) {
	metaFilePath := gridFilePath + ".meta"

	file, err := os.OpenFile(metaFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	file.WriteString(fmt.Sprintf("title: '%s'\n", title))
	file.WriteString("yTitle: 1.00\n")
	file.WriteString("xTitle: 0.00\n")
	file.WriteString("removeEmptyColumns: True\n")
	file.WriteString(fmt.Sprintf("labeltext: '%s'\n", labeltext))
	if colormap != "" {
		file.WriteString(fmt.Sprintf("colormap: '%s'\n", colormap))
	}
	if colorlist != nil {
		file.WriteString("colorlist: \n")
		for _, item := range colorlist {
			file.WriteString(fmt.Sprintf(" - '%s'\n", item))
		}
	}
	if cbarLabel != nil {
		file.WriteString("cbarLabel: \n")
		for _, cbarItem := range cbarLabel {
			file.WriteString(fmt.Sprintf(" - '%s'\n", cbarItem))
		}
	}
	if ticklist != nil {
		file.WriteString("ticklist: \n")
		for _, tick := range ticklist {
			file.WriteString(fmt.Sprintf(" - %f\n", tick))
		}
	}
	if len(colorlistType) > 0 {
		file.WriteString(fmt.Sprintf("colorlisttype: %s\n", colorlistType))
	}
	file.WriteString(fmt.Sprintf("factor: %f\n", factor))
	if maxValue != nodata {
		file.WriteString(fmt.Sprintf("maxValue: %0.2f\n", maxValue))
	}
	if minValue != nodata {
		file.WriteString(fmt.Sprintf("minValue: %0.2f\n", minValue))
	}
	if len(minColor) > 0 {
		file.WriteString(fmt.Sprintf("minColor: %s\n", minColor))
	}

	file.WriteString("yLabel: 'Latitude'\n")
	file.WriteString("YaxisMappingFile: 'stacked_y_lat_buckets.csv'\n")
	file.WriteString("YaxisMappingRefColumn: Bucket\n")
	file.WriteString("YaxisMappingTarColumn: Latitude\n")
	file.WriteString("YaxisMappingFormat: '{:2.0f}°'\n")
	file.WriteString("yTicklist: \n")
	file.WriteString("- 8\n")
	file.WriteString("- 21\n")
	file.WriteString("- 35\n")
	file.WriteString("- 49\n")

}

func createGridFile(name string, header *AsciiGridMeta) (*Fout, error) {
	cornerX := header.XllCorner
	cornery := header.YllCorner
	novalue := header.NoDataValue
	cellsize := header.CellSize

	fout, err := createGzFileWriter(name)
	if err != nil {
		return nil, err
	}

	fout.Write(fmt.Sprintf("ncols %d\n", header.NCols))
	fout.Write(fmt.Sprintf("nrows %d\n", header.NRows))
	fout.Write(fmt.Sprintf("xllcorner     %f\n", cornerX))
	fout.Write(fmt.Sprintf("yllcorner     %f\n", cornery))
	fout.Write(fmt.Sprintf("cellsize      %f\n", cellsize))
	fout.Write(fmt.Sprintf("NODATA_value  %f\n", novalue))

	return fout, nil
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
