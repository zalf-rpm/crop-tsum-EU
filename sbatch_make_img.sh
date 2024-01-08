#!/bin/bash -x
#SBATCH --partition=compute
#SBATCH --job-name=img_gen_ascii
#SBATCH --output=img_gen_ascii-%j.out
#SBATCH --time=01:10:00
#SBATCH --ntasks=1
#SBATCH --cpus-per-task=80

FOLDER=$( pwd )
IMG=~/singularity/python/python3.7_2.0.sif
singularity run -B $FOLDER:$FOLDER $IMG python create_image_from_ascii.py source=historical out=historical &
singularity run -B $FOLDER:$FOLDER $IMG python create_image_from_ascii.py source=2_GFDL-CM3_45 out=2_GFDL-CM3_45 &
singularity run -B $FOLDER:$FOLDER $IMG python create_image_from_ascii.py source=2_GFDL-CM3_85 out=2_GFDL-CM3_85 &
singularity run -B $FOLDER:$FOLDER $IMG python create_image_from_ascii.py source=2_GISS-E2-R_45 out=2_GISS-E2-R_45 &
singularity run -B $FOLDER:$FOLDER $IMG python create_image_from_ascii.py source=2_GISS-E2-R_85 out=2_GISS-E2-R_85 &
singularity run -B $FOLDER:$FOLDER $IMG python create_image_from_ascii.py source=2_HadGEM2-ES_45 out=2_HadGEM2-ES_45 &
singularity run -B $FOLDER:$FOLDER $IMG python create_image_from_ascii.py source=2_HadGEM2-ES_85 out=2_HadGEM2-ES_85 &
singularity run -B $FOLDER:$FOLDER $IMG python create_image_from_ascii.py source=2_MIROC5_45 out=2_MIROC5_45 &
singularity run -B $FOLDER:$FOLDER $IMG python create_image_from_ascii.py source=2_MIROC5_85 out=2_MIROC5_85 &
singularity run -B $FOLDER:$FOLDER $IMG python create_image_from_ascii.py source=2_MPI-ESM-MR_45 out=2_MPI-ESM-MR_45 &
singularity run -B $FOLDER:$FOLDER $IMG python create_image_from_ascii.py source=2_MPI-ESM-MR_85 out=2_MPI-ESM-MR_85 &

wait