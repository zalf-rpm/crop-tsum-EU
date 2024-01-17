#!/bin/bash -x
#SBATCH --partition=compute
#SBATCH --job-name=img_gen_ascii
#SBATCH --output=img_gen_ascii-%j.out
#SBATCH --time=01:10:00
#SBATCH --ntasks=1
#SBATCH --cpus-per-task=80

CROP=$1 #crop
mkdir -p img/${CROP}
FOLDER=$( pwd )
IMG=~/singularity/python/python3.7_2.0.sif
singularity run -B $FOLDER:$FOLDER $IMG python gen_img/create_image_from_ascii.py source=crops/${CROP}/historical out=img/${CROP} &
singularity run -B $FOLDER:$FOLDER $IMG python gen_img/create_image_from_ascii.py source=crops/${CROP}/2_GFDL-CM3_45 out=img/${CROP} &
singularity run -B $FOLDER:$FOLDER $IMG python gen_img/create_image_from_ascii.py source=crops/${CROP}/2_GFDL-CM3_85 out=img/${CROP} &
singularity run -B $FOLDER:$FOLDER $IMG python gen_img/create_image_from_ascii.py source=crops/${CROP}/2_GISS-E2-R_45 out=img/${CROP} &
singularity run -B $FOLDER:$FOLDER $IMG python gen_img/create_image_from_ascii.py source=crops/${CROP}/2_GISS-E2-R_85 out=img/${CROP} &
singularity run -B $FOLDER:$FOLDER $IMG python gen_img/create_image_from_ascii.py source=crops/${CROP}/2_HadGEM2-ES_45 out=img/${CROP} &
singularity run -B $FOLDER:$FOLDER $IMG python gen_img/create_image_from_ascii.py source=crops/${CROP}/2_HadGEM2-ES_85 out=img/${CROP} &
singularity run -B $FOLDER:$FOLDER $IMG python gen_img/create_image_from_ascii.py source=crops/${CROP}/2_MIROC5_45 out=img/${CROP} &
singularity run -B $FOLDER:$FOLDER $IMG python gen_img/create_image_from_ascii.py source=crops/${CROP}/2_MIROC5_85 out=img/${CROP} &
singularity run -B $FOLDER:$FOLDER $IMG python gen_img/create_image_from_ascii.py source=crops/${CROP}/2_MPI-ESM-MR_45 out=img/${CROP} &
singularity run -B $FOLDER:$FOLDER $IMG python gen_img/create_image_from_ascii.py source=crops/${CROP}/2_MPI-ESM-MR_85 out=img/${CROP} &

wait