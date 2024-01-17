package main

import "testing"

func Test_generateCropFile(t *testing.T) {
	type args struct {
		cropFileName string
	}
	cropTempl := Crop{
		Name:         "soybean_0",
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

	tests := []struct {
		name string
		args args
	}{
		{"test1", args{cropFileName: "soybean.yml"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := generateCropFile(tt.args.cropFileName)
			if err != nil {
				t.Errorf("generateCropFile() error = %v", err)
				return
			}
			crop, err := readCropData(tt.args.cropFileName)
			if err != nil {
				t.Errorf("readCropData() error = %v", err)
				return
			}
			if crop.Name != cropTempl.Name {
				t.Errorf("crop.Name = %v, want %v", crop.Name, cropTempl.Name)
			}
			if crop.TsumMaturity != cropTempl.TsumMaturity {
				t.Errorf("crop.TsumMaturity = %v, want %v", crop.TsumMaturity, cropTempl.TsumMaturity)
			}
			if crop.FrostTreashold != cropTempl.FrostTreashold {
				t.Errorf("crop.FrostTreashold = %v, want %v", crop.FrostTreashold, cropTempl.FrostTreashold)
			}
			for i, stage := range crop.Stages {
				if stage.Name != cropTempl.Stages[i].Name {
					t.Errorf("stage.Name = %v, want %v", stage.Name, cropTempl.Stages[i].Name)
				}
				if stage.Tsum != cropTempl.Stages[i].Tsum {
					t.Errorf("stage.Tsum = %v, want %v", stage.Tsum, cropTempl.Stages[i].Tsum)
				}
				if stage.BaseTemp != cropTempl.Stages[i].BaseTemp {
					t.Errorf("stage.BaseTemp = %v, want %v", stage.BaseTemp, cropTempl.Stages[i].BaseTemp)
				}
			}

		})
	}
}
