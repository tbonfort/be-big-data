import base64
import json
import base64
import rasterio
from rasterio.windows import Window
from rasterio.transform import Affine
import numpy as np
import multiprocessing.dummy as mp
from functools import partial
import tempfile
import os
from google.cloud import storage
from google.cloud.storage.blob import Blob
import numba
import time


from flask import Flask, request


@numba.jit()
def median(inputs):
    bandLen = len(inputs[0]) // 3
    sample = np.zeros((len(inputs), 3), dtype=np.uint8)
    #sample = [[0,0,0]] * len(inputs)
    result = np.zeros_like(inputs[0])
    for pix in range(bandLen):
        count = 0
        for s in range(len(inputs)):
            if (inputs[s][pix] == 0 or inputs[s][pix+bandLen] == 0 or inputs[s][pix+2*bandLen] == 0 or
                inputs[s][pix] == 255 or inputs[s][pix+bandLen] == 255 or inputs[s][pix+2*bandLen] == 255): #nodata or saturated
                continue
            sample[count] = [inputs[s][pix], inputs[s][pix+bandLen], inputs[s][pix+2*bandLen]]
            count += 1
        if count == 0:
            continue
        shortlist = list(sample[:count])
        shortlist.sort(key=lambda x: sum(x))
        med = count // 2
        result[pix], result[pix+bandLen], result[pix+2*bandLen] = shortlist[med]
    return result

def getbuffer(dataset, box):
    with rasterio.open(dataset) as ds:
        if ds.count != 3:
            raise ValueError("expecting 3 bands, got %d" % ds.count)
        if any(x < 0 for x in box) or box[0]+box[2] > ds.width or box[1]+box[3] > ds.height:
            raise ValueError("window out of bounds")
        buf = ds.read(window=Window(box[0],box[1],box[2],box[3]))#,out_shape=(box[2],box[3],3))
        return buf.flatten()


app = Flask(__name__)

@app.route("/median", methods=["POST"])
def index():
    """Receive and parse Pub/Sub messages."""
    envelope = request.get_json()
    if not envelope:
        msg = "no Pub/Sub message received"
        print(f"error: {msg}")
        return f"Bad Request: {msg}", 400

    if not isinstance(envelope, dict) or "message" not in envelope:
        msg = "invalid Pub/Sub message format"
        print(f"error: {msg}")
        return f"Bad Request: {msg}", 400

    
    r = json.loads(base64.b64decode(envelope['message']['data']))

    start = time.time()
    getbuffer_part = partial(getbuffer, box=r["window"])
    with mp.Pool(10) as pool:
        bufs = pool.map(getbuffer_part, r["datasets"])
    print("read time", time.time() - start)

    start = time.time()
    result = median(bufs)
    print("median time", time.time() - start)



    with rasterio.open(r["datasets"][0]) as ds0:
        profile = ds0.profile
        old_gt = ds0.transform
        #print(ds0.profile)

    new_gt = Affine(old_gt.a, old_gt.b, old_gt.c + r["window"][0] * old_gt.a, old_gt.d, old_gt.e, old_gt.f + r["window"][1] * old_gt.e)
    

    profile["transform"] = new_gt
    profile["width"] = r["window"][2]
    profile["height"] = r["window"][3]
    profile['driver'] = 'COG'
    profile['tiled'] = True
    profile['interleaved'] = 'pixel'
    profile['blocksize'] = 256
    profile['photometric'] = 'YCbCr'
    profile['compress'] = 'JPEG'
    profile['quality'] = 90

    f,name = tempfile.mkstemp(suffix=".tif")

    try:
        with rasterio.open(name, 'w', **profile) as dst:
            dst.write(result.reshape((3, r["window"][3], r["window"][2])))
        st = storage.Client(project="solar-climber-412810")
        blob = Blob.from_string(r["destination"], st)
        blob.upload_from_filename(name)
        print(f"uploaded to {r['destination']}")
    finally:
        os.remove(name)
    





    
    return ("", 204)

