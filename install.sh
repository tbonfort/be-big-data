#!/bin/bash

#echo HOME=$HOME
#echo USER=$USER
## install gdal gs:// handler
#git clone -q https://github.com/airbusgeo/godal
#cd godal/gdalplugin
#make
#sudo cp gdal_gcs.so /usr/lib/x86_64-linux-gnu/gdalplugins/
#cd ../..
#rm -rf godal
echo export GOOGLE_APPLICATION_CREDENTIALS=$HOME/.config/gcloud/application_default_credentials.json >> $HOME/.bashrc
echo export GODAL_NUMBLOCKS=1000 >> $HOME/.bashrc
echo export GODAL_BLOCKSIZE=128k >> $HOME/.bashrc


