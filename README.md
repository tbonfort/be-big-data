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

# initial setup

- Create a codespace from this repository: click on the green `<> Code` button
  near the top right of the file list, select the `codespaces` tab, then click
  on the green `create codespace on master` button
- wait for the codespace to boot and the setup script to finish
- authenticate with GCP from the codespace terminal: `gcloud auth login`

once the setup has finished, open the `be.ipynb` notebook to explore the available data
before continuing to the next section








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

# reconstructing/viewing the final median image

c.f. notebook

# bonus

TODO: robustify and secure

# cleanup
TODO: delete gcp ressources created during this BE
