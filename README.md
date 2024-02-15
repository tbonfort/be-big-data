# introduction
This course will introduce running embarassingly parallel computations on 
sentinel-2 rgb imagery using google cloud run. The (very artificial) objective 
is to compute the median image from a one-year (2023) time-series over a given
s2 granule.

You will find the RGB timeseries data in COG format in gs://tb-be-bigdata/t31tcj
which is publicly accessible. This timeseries contains 216 scenes and weighs in at
over 22Gb of data.

Instead of processing this volume of data on a single node, we will be splitting
up the workload over multiple independant workers, each worker being responsible
for computing the median pixel values over a small window (e.g. 1024x1024 px). The
full job which consists of processing 216x10980x10980 pixels can in this case
be decomposed into ~110 independant jobs each having to process only 216x1024x1024
pixels.

Load the `be.ipynb` notebook to explore the available data.

# config

```bash
#your gcp project
export GCPPROJECT=foo-bar-1234
#a globally unique bucket name for this BE
export BUCKETNAME=xxxxx

export DIMAGE=eu.gcr.io/$GCPPROJECT/be:202401
export MYNAME=bebigdata
export SAEMAIL=$MYNAME@$GCPPROJECT.iam.gserviceaccount.com

gcloud auth login
export GOOGLE_APPLICATION_CREDENTIALS=$HOME/.config/gcloud/myemail@mydomain.com/adc.json

gcloud --project=$GCPPROJECT iam service-accounts create $MYNAME
gcloud --project=$GCPPROJECT services enable containerregistry.googleapis.com
gcloud --project=$GCPPROJECT services enable run.googleapis.com
gcloud --project=$GCPPROJECT services enable pubsub.googleapis.com
gcloud --project=$GCPPROJECT auth configure-docker
```

```bash
gcloud --project=$GCPPROJECT storage buckets create gs://$BUCKETNAME --default-storage-class=standard --location=europe-west1
gcloud --project=$GCPPROJECT storage buckets add-iam-policy-binding gs://$BUCKETNAME \
--member=serviceAccount:$SAEMAIL \
--role=roles/storage.objectAdmin
```

# deploying the cloud run worker

we will be creating a docker image that exposes an HTTP endpoint that receives
request with a payload of the form

```json
{
    "window":[x0,y0,width,height],
    "destination":"gs://bucket/path-to-result-tile.tif",
    "datasets":["gs://bucket/prefix/T31TCJ_20230102T104441_TCI.tif",
        "gs://bucket/prefix/T31TCJ_20230103T110349_TCI.tif",
        "....and 214 more..."]
}
```
upon receiving an HTTP request, the worker should:

- decode the json payload
- for each dataset, extract the buffer of shape (3,width,height) starting at (x0,y0)
- create a resulting buffer of shape (3,width,height) where each pixel corresponds to
  the rgb triplet of median luminance, after having filtered out samples that are equal
  to 0 (no data, i.e. outide the satellite swath) or 255 (saturated, most likely cloud)
- upload the resulting buffer as a COG file to the requested destination



## docker

create the program that will be used for computing individual tiles.

An example implementation can be found in the `answers/worker` directory,
which can be tested locally on a single tile by using the `dispatcher` code
from the notebook, and running a local webserver with:

```bash
gunicorn --bind :8080 --workers 1 --threads 1 --timeout 0 main:app --reload
```

Once your code is working correctly, build it as a docker image so that it can
be hosted on another platform

```bash
docker build -t $DIMAGE .
docker push $DIMAGE
```

## cloud run


```bash
gcloud --project=$GCPPROJECT run deploy $MYNAME --image $DIMAGE --allow-unauthenticated \
--service-account=$SAEMAIL --region=europe-west1 \
--set-env-vars=CPL_MACHINE_IS_GCE=YES \
--memory 2048M --concurrency=1 --max-instances=50
```
The command will print out on which URL your service is listening.
```bash
# you MUST edit this
export RUNURL=https://bebigdata-xxxx-yyyy-zzzz.a.run.app
```

## pubsub
We will now configure a pubsub queue which is configured to dispatch payloads
to our cloud run service:

```bash
gcloud --project=$GCPPROJECT pubsub topics create $MYNAME
gcloud --project=$GCPPROJECT pubsub topics create myerrors
gcloud --project=$GCPPROJECT pubsub subscriptions create $MYNAME --topic $MYNAME \
--ack-deadline=600 \
--max-delivery-attempts=5 --dead-letter-topic=myerrors \
--push-endpoint=$RUNURL/median
```
go to the pubsub console page and allow/adjust service account rights on myerrors topic

# launching a parallel job

the code in the `dispatch` section of the notebook creates one payload per tile
covering the t31tcj granule and publishes it to pubsub


go to the cloud run console and observe the metrics and logs (that can take a few
seconds to appear). if all worked correctly, we can now launch the full number of
jobs.

check your output bucket (by default $BUCKETNAME/results): are all tiles present?

TODO: check log for errors. were retries successful? how to configure cloud-run
concurrency and/or pubsub retries to avoid errors and/or 

# reconstructing the final median image

c.f. notebook

# bonus

TODO: robustify and secure

# cleanup
TODO: delete gcp ressources created during this BE
