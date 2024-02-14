import base64
import json
import base64
import rasterio
from rasterio.windows import Window
import numpy as np

from flask import Flask, request


def median(inputs):
    bandLen = len(inputs[0]) // 3
    sample = np.zeros((len(inputs), 3), dtype=np.uint8)
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
        #print(sample[:count])
        #print(np.sum(sample[:count], axis=1))
        short=list(sample[:count])
        short.sort(key=lambda x: sum(x))
        #sample[:count] = list(sample[:count]).sort(key=lambda x: sum(x))
        med = count // 2
        result[pix], result[pix+bandLen], result[pix+2*bandLen] = short[med]
    return result

def getbuffer(dataset, box):
    with rasterio.open(dataset) as ds:
        if ds.count != 3:
            raise ValueError("expecting 3 bands, got %d" % ds.count)
        if any(x < 0 for x in box) or box[0]+box[2] > ds.width or box[1]+box[3] > ds.height:
            raise ValueError("window out of bounds")
        buf = ds.read(window=Window(box[0],box[1],box[2],box[3]))
        return buf.flatten()

def getbuffers(datasets, window):
    return [getbuffer(ds, window) for ds in datasets]


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

    bufs = getbuffers(r["datasets"], r["window"])
    result = median(bufs)
    with rasterio.open(r["datasets"][0]) as ds0:
        profile = ds0.profile
    profile["width"] = r["window"][2]
    profile["height"] = r["window"][3]
    with rasterio.open(r["destination"], 'w', **profile) as dst:
        dst.write(result.reshape((3, r["window"][3], r["window"][2])))





    
    return ("", 204)

