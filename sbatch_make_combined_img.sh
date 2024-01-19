#!/bin/bash -x
#SBATCH --partition=compute
#SBATCH --job-name=img_gen_ascii
#SBATCH --output=img_gen_ascii-%j.out
#SBATCH --time=01:10:00
#SBATCH --ntasks=1
#SBATCH --cpus-per-task=80

#list of crops
CROPS="buckwheat caraway chickpea_w1988 durum grass_pea l_albus l_angustifolius lentil millet sesame sorghum soybean tomato upland_rice"

# for each crop and crop path
for CROP in $CROPS; do
    ./combine/combine -config ./combine/config.yml -crop-path ${CROP} -crop ${CROP} &
done

wait

# create images from ascii
mkdir -p img/combined
FOLDER=$( pwd )
IMG=~/singularity/python/python3.7_2.0.sif
singularity run -B $FOLDER:$FOLDER $IMG python gen_img/create_image_from_ascii.py source=crops/combined out=img/combined

