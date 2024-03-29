# introduction

lesson slides: 
https://docs.google.com/presentation/d/1Lc89oWizg0-6ZJm7UlNsq3n90FxxhUKRgCgZoZMQq14/edit?usp=sharing

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

# initial setup

- Create a codespace from this repository: click on the green `<> Code` button
  near the top right of the file list, select the `codespaces` tab, then click
  on the green `create codespace on master` button
- wait for the codespace to boot and the setup script to finish
- authenticate with GCP from the codespace terminal: `gcloud auth login`

once the setup has finished, open the `be.ipynb` notebook to explore the available data
before continuing to the next section

# config

edit the following code where noted, and copy paste the commands into your codespace
terminal in order to setup our local environment and the GCP ressources we will be using.

```bash
# there are 3 variables you must edit before running the subsequent commands:

# replace with your gcp project name
export GCPPROJECT=foo-bar-1234
# edit and choose a globally unique bucket name for this BE
export BUCKETNAME=xxxxx
# edit to point to the correct file (which was created when you ran "gcloud auth login")
export GOOGLE_APPLICATION_CREDENTIALS=$HOME/.config/gcloud/legacy_credentials/myemail@mydomain.com/adc.json

# you may edit the following variables, but it is not required
# the name of the docker image to produce
export DIMAGE=eu.gcr.io/$GCPPROJECT/be:202401
# the name of the pubsub queue and cloud run service we will create
export MYNAME=bebigdata
# name of the service account running the cloud run service, which we must authorize to 
# read and create data on your cloud bucket
export SAEMAIL=$MYNAME@$GCPPROJECT.iam.gserviceaccount.com


gcloud --project=$GCPPROJECT iam service-accounts create $MYNAME
gcloud --project=$GCPPROJECT services enable containerregistry.googleapis.com
gcloud --project=$GCPPROJECT services enable run.googleapis.com
gcloud --project=$GCPPROJECT services enable pubsub.googleapis.com
gcloud --project=$GCPPROJECT auth configure-docker
```

create a bucket to store our data. in case this command errors out, choose another name
for $BUCKETNAME
```bash
gcloud --project=$GCPPROJECT storage buckets create gs://$BUCKETNAME --default-storage-class=standard --location=europe-west1
gcloud --project=$GCPPROJECT storage buckets add-iam-policy-binding gs://$BUCKETNAME \
--member=serviceAccount:$SAEMAIL \
--role=roles/storage.objectAdmin
```

# creating the worker code

we will be creating a program that exposes an HTTP endpoint that receives
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
upon receiving an HTTP request, the program should:

- decode the json payload
- for each dataset, extract the buffer of shape (3,width,height) starting at (x0,y0)
- create a resulting buffer of shape (3,width,height) where each pixel corresponds to
  the rgb triplet of median luminance, after having filtered out samples that are equal
  to 0 (no data, i.e. outide the satellite swath) or 255 (saturated, most likely cloud)
- upload the resulting buffer as a COG file to the requested destination



## implementation

An example implementation can be found in the `worker.py` file.

we will first test this code locally on a single tile.
return to the `dispatcher` code from the notebook to understand how we will be
creating json payloads that can be processed by our worker program.

from a terminal, run a local webserver that exposes our worker code with:

```bash
gunicorn --bind :8080 --workers 1 --threads 1 --timeout 0 worker:app --reload
```

and then run the notebook's dispatch block. check the logs of our worker and
make sure that our resulting tile is now available on our bucket.


## docker

https://docs.docker.com/get-started/overview/#the-docker-platform

Once your code is working correctly, build it as a docker image so that it can
be hosted on another platform. 

```bash
docker build -t $DIMAGE .
docker push $DIMAGE
```

we can also run this code locally directly from the docker image, on the :8081
port:

```bash
docker run -t -e PORT=8081 -p 8081:8081 $DIMAGE
```
modify the dispatch code so that your test request is sent to this docker instance.
this time the request will fail. why did it fail?

bonus: modify the previous `docker run` command so that it runs correctly. hint: use
a docker volume to mount your local credentials, and set the correct environment variable
to point to the local path of your credential file.

## cloud run

Cloud Run is a managed compute platform that lets you run containers directly on top of Google's
scalable infrastructure. You can deploy code written in any programming language on Cloud Run
if you can build a container image from it.

https://cloud.google.com/run/docs/overview/what-is-cloud-run

https://console.cloud.google.com/run


```bash
gcloud --project=$GCPPROJECT run deploy $MYNAME --image $DIMAGE --allow-unauthenticated \
--service-account=$SAEMAIL --region=europe-west1 \
--set-env-vars=CPL_MACHINE_IS_GCE=YES \
--memory 2048M --concurrency=1 --max-instances=50
```
The command will print out on which URL your service is listening.
```bash
# you MUST edit this to replace with the url printed out
export RUNURL=https://bebigdata-xxxx-yyyy-zzzz.a.run.app
```

Adapt the dispatch code to stop sending http requests to localhost, but instead
point them to the $RUNURL/median endpoint on cloud run. Post a single tile to
that endpoint and check the cloud run logs that the tile has been processed
without errors.

This time the code running from the exact same docker image did not fail with permission
errors, why? https://cloud.google.com/run/docs/securing/service-identity

## pubsub

Pub/Sub is an asynchronous and scalable messaging service that decouples services
producing messages from services processing those messages.

https://cloud.google.com/pubsub/docs/overview


We will now create a pubsub queue which is configured to dispatch payloads
to our cloud run service. Each time a message is posted to this pubsub queue,
the pubsub service will emit a request to our cloud run endpoint, which in turn
will cause cloud run to create an instance to process that request/payload.

```bash
gcloud --project=$GCPPROJECT pubsub topics create $MYNAME
gcloud --project=$GCPPROJECT pubsub topics create myerrors

#before running this, make sure you have updated $RUNURL with the correct value 
#that was printed out when you deployed your cloud run service
gcloud --project=$GCPPROJECT pubsub subscriptions create $MYNAME --topic $MYNAME \
--ack-deadline=600 \
--max-delivery-attempts=5 --dead-letter-topic=myerrors \
--push-endpoint=$RUNURL/median
```

optional: go to the pubsub console page and allow/adjust service account rights on myerrors topic

https://console.cloud.google.com/cloudpubsub/subscription/detail/bebigdata


# launching a parallel job

switch back again to the notebook, to the `dispatcher` section

adapt the code once again to stop sending http requests and instead push its payloads
to pubsub. Post a single tile payload to the pubsub queue, and check again
the cloud run logs that the tile has been processed without errors.

once there are no errors, adapt the code so that **all** tiles covering
the t31tcj granule are published to pubsub.

**WARNING**: this will cause a large number of instances to be booted up and billed.
you can always cancel a failing batch of requests by navigating to your pubsub queue
at https://console.cloud.google.com/cloudpubsub/subscription/detail/bebigdata and
clicking the `purge message` button.

go to the cloud run console and observe the metrics and logs (they can take a few
seconds to appear).

check your output bucket (by default $BUCKETNAME/results): are all tiles present?

TODO: check log for errors. were retries successful? how to configure cloud-run
concurrency and/or pubsub retries to avoid errors and/or 

how much compute was provisionned for this exercise? how long would the whole
process have taken if we only had a single instance available? what parameters
would need changing if we had decided to use a different tiling scheme to split
the job (with larger tiles? with smaller tiles?)

# reconstructing/viewing the final median image

c.f. notebook

# bonus

TODO: robustify and secure

# cleanup
delete gcp ressources created during this BE:

- delete the bucket associated with this course
- delete the pubsub topic and subcription
- delete the cloud-run deployment
- delete the docker image we pushed
