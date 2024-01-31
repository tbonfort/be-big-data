config
======

```bash
export GCPPROJECT=foo-bar-1234
export BUCKETNAME=xxxxx

export DIMAGE=eu.gcr.io/$GCPPROJECT/be:202401
export MYNAME=bebigdata
export SAEMAIL=$MYNAME@$GCPPROJECT.iam.gserviceaccount.com

gcloud iam service-accounts create $MYNAME
gcloud services enable containerregistry.googleapis.com
gcloud services enable run.googleapis.com
gcloud services enable pubsub.googleapis.com
gcloud auth configure-docker
```

```bash
gcloud storage buckets create gs://$BUCKETNAME --default-storage-class=standard --location=europe-west1
gcloud storage buckets add-iam-policy-binding gs://$BUCKETNAME --member=serviceAccount:$SAEMAIL --role=roles/storage.objectAdmin
```

# deploying the cloud run worker

## docker

create the docker image that will be deployed for computing individual tiles:
<details>
  <summary>
    Dockerfile
  </summary>
  
```dockerfile
FROM golang:bookworm AS builder
RUN apt update && apt -y install libgdal-dev
WORKDIR /build
COPY main.go /build
RUN go mod init be && go mod tidy && go build -o be .
  
FROM debian:bookworm
RUN apt update && install -y libgdal32 && rm -rf /var/lib/{apt,dpkg,cache,log}
COPY --from=builder /build/be /be
ENTRYPOINT /be
```
  
</details>

build and deploy this image as a cloud run service

```bash
docker build -t $DIMAGE .
docker push $DIMAGE
```

## cloud run

```bash
gcloud run deploy $MYNAME --image $DIMAGE --allow-unauthenticated \
--service-account=$SAEMAIL --region=europe-west1 \
--set-env-vars=CPL_MACHINE_IS_GCE=YES \
--memory 2048M --concurrency=1 --max-instances=50
```
The command will print out on which URL your service is listening.
```bash
export RUNURL=https://bebigdata-xxxx-yyyy-zzzz.a.run.app
```

## pubsub

```bash
gcloud pubsub topics create $MYNAME
gcloud pubsub topics create myerrors
gcloud pubsub subscriptions create $MYNAME --topic $MYNAME \
--ack-deadline=600 \
--max-delivery-attempts=5 --dead-letter-topic=myerrors \
--push-endpoint=$RUNURL/median
```
go to the pubsub console page and allow/adjust service account rights on myerrors topic
