#!/bin/bash

# install gdal gs:// handler
git clone https://github.com/airbusgeo/godal
cd godal/gdalplugin
make
cp gdal_gcs.so /usr/lib/x86_64-linux-gnu/gdalplugins/
cd ../..
rm -rf godal
#echo "export GOOGLE_APPLICATION_CREDENTIALS="$HOME/.config/gcloud/legacy_credentials/*/adc.json >> /etc/bash.bashrc
echo export GODAL_NUMBLOCKS=1000 >> /etc/bash.bashrc
echo export GODAL_BLOCKSIZE=128k >> /etc/bash.bashrc


