import base64
import json
import base64
import rasterio
from rasterio.windows import Window
from rasterio.transform import Affine
import numpy as np
import numpy.ma as ma
import multiprocessing.dummy as mp
from functools import partial
import tempfile
import os
from google.cloud import storage
from google.cloud.storage.blob import Blob


from flask import Flask, request


def median(inputs):
    bandLen = len(inputs[0]) // 3
    inputs = np.array(inputs)
    inputs = inputs.reshape(-1, 3, bandLen)
    result = np.median(inputs, axis=0)
    return result.flatten().astype(np.uint8)

def median_nodata(inputs):
    n_bands = 3
    bandLen = len(inputs[0]) // n_bands
    inputs = np.array(inputs, dtype=np.float16)
    inputs = inputs.reshape(-1, n_bands, bandLen)
    # mask for a pixel for an image, any band is 0 or 255
    mask = np.any((inputs == 0) | (inputs == 255), axis=1)
    # duplicate the mask for the "n_bands"" bands
    mask_full = np.repeat(mask, n_bands, axis=1).reshape(-1, n_bands, bandLen, order='F')
    # compute the masked median
    inputs_ma = ma.masked_array(inputs, mask_full)
    result = ma.median(inputs_ma, axis=0).filled(0)
    return result.flatten().astype(np.uint8)

    

def getbuffer(dataset, box):
    with rasterio.open(dataset) as ds:
        if ds.count != 3:
            raise ValueError("expecting 3 bands, got %d" % ds.count)
        if any(x < 0 for x in box) or box[0]+box[2] > ds.width or box[1]+box[3] > ds.height:
            raise ValueError("window out of bounds")
        buf = ds.read(window=Window(box[0],box[1],box[2],box[3]))
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


    getbuffer_part = partial(getbuffer, box=r["window"])
    with mp.Pool(10) as pool:
        bufs = pool.map(getbuffer_part, r["datasets"])

    result = median_nodata(bufs)



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

