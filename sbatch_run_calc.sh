#!/bin/bash -x
#SBATCH --partition=compute
#SBATCH --job-name=calc_tsum
#SBATCH --output=calc_tsum-%j.out
#SBATCH --time=01:10:00
#SBATCH --ntasks=1
#SBATCH --cpus-per-task=80

# climate scenarios
CLIMATE=$1 # climate scenario root folder
GRID_REF=$2 # grid to reference file folder
CROP=$3 #crop

# remove ext from crop file
CROPOUT=${CROP%.*}

PROGRAM=./${PROGRAM}/crop-tsum-EU
mkdir -p $CROPOUT

${PROGRAM} \
-crop ${CROP} \
-sowing sowing_dates/0_0_sowing-dates.csv.gz \
-weather ${CLIMATE}/0/0_0/%s_v3.csv \
-reference ${GRID_REF}/stu_eu_layer_ref.csv \
-grid_to_ref ${GRID_REF}/stu_eu_layer_grid.csv \
-output $CROPOUT/historical &

${PROGRAM} \
-crop ${CROP} \
-sowing sowing_dates/GFDL-CM3_45_sowing-dates.csv.gz \
-weather ${CLIMATE}/2/GFDL-CM3_45/%s_v3.csv \
-reference ${GRID_REF}/stu_eu_layer_ref.csv \
-grid_to_ref ${GRID_REF}/stu_eu_layer_grid.csv \
-output $CROPOUT/2_GFDL-CM3_45 &

${PROGRAM} \
-crop ${CROP} \
-sowing sowing_dates/GFDL-CM3_85_sowing-dates.csv.gz \
-weather ${CLIMATE}/2/GFDL-CM3_85/%s_v3.csv \
-reference ${GRID_REF}/stu_eu_layer_ref.csv \
-grid_to_ref ${GRID_REF}/stu_eu_layer_grid.csv \
-output $CROPOUT/2_GFDL-CM3_85 &

${PROGRAM} \
-crop ${CROP} \
-sowing sowing_dates/GISS-E2-R_45_sowing-dates.csv.gz \
-weather ${CLIMATE}/2/GISS-E2-R_45/%s_v3.csv \
-reference ${GRID_REF}/stu_eu_layer_ref.csv \
-grid_to_ref ${GRID_REF}/stu_eu_layer_grid.csv \
-output $CROPOUT/2_GISS-E2-R_45 &

${PROGRAM} \
-crop ${CROP} \
-sowing sowing_dates/GISS-E2-R_85_sowing-dates.csv.gz \
-weather ${CLIMATE}/2/GISS-E2-R_85/%s_v3.csv \
-reference ${GRID_REF}/stu_eu_layer_ref.csv \
-grid_to_ref ${GRID_REF}/stu_eu_layer_grid.csv \
-output $CROPOUT/2_GISS-E2-R_85 &


${PROGRAM} \
-crop ${CROP} \
-sowing sowing_dates/HadGEM2-ES_45_sowing-dates.csv.gz \
-weather ${CLIMATE}/2/HadGEM2-ES_45/%s_v3.csv \
-reference ${GRID_REF}/stu_eu_layer_ref.csv \
-grid_to_ref ${GRID_REF}/stu_eu_layer_grid.csv \
-output $CROPOUT/2_HadGEM2-ES_45 &

${PROGRAM} \
-crop ${CROP} \
-sowing sowing_dates/HadGEM2-ES_85_sowing-dates.csv.gz \
-weather ${CLIMATE}/2/HadGEM2-ES_85/%s_v3.csv \
-reference ${GRID_REF}/stu_eu_layer_ref.csv \
-grid_to_ref ${GRID_REF}/stu_eu_layer_grid.csv \
-output $CROPOUT/2_HadGEM2-ES_85 &


${PROGRAM} \
-crop ${CROP} \
-sowing sowing_dates\MIROC5_45_sowing-dates.csv.gz \
-weather ${CLIMATE}/2/MIROC5_45/%s_v3.csv \
-reference ${GRID_REF}/stu_eu_layer_ref.csv \
-grid_to_ref ${GRID_REF}/stu_eu_layer_grid.csv \
-output $CROPOUT/2_MIROC5_45 &

${PROGRAM} \
-crop ${CROP} \
-sowing sowing_dates\MIROC5_85_sowing-dates.csv.gz \
-weather ${CLIMATE}/2/MIROC5_85/%s_v3.csv \
-reference ${GRID_REF}/stu_eu_layer_ref.csv \
-grid_to_ref ${GRID_REF}/stu_eu_layer_grid.csv \
-output $CROPOUT/2_MIROC5_85 &

${PROGRAM} \
-crop ${CROP} \
-sowing sowing_dates\MPI-ESM-MR_45_sowing-dates.csv.gz \
-weather ${CLIMATE}/2/MPI-ESM-MR_45/%s_v3.csv \
-reference ${GRID_REF}/stu_eu_layer_ref.csv \
-grid_to_ref ${GRID_REF}/stu_eu_layer_grid.csv \
-output $CROPOUT/2_MPI-ESM-MR_45 &

${PROGRAM} \
-crop ${CROP} \
-sowing sowing_dates\MPI-ESM-MR_85_sowing-dates.csv.gz \
-weather ${CLIMATE}/2/MPI-ESM-MR_85/%s_v3.csv \
-reference ${GRID_REF}/stu_eu_layer_ref.csv \
-grid_to_ref ${GRID_REF}/stu_eu_layer_grid.csv \
-output $CROPOUT/2_MPI-ESM-MR_85 &

wait
