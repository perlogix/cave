#!/usr/bin/env python3

import requests
import json
import string
import random
import sys
import uuid

URL = "http://{}/api/v1/kv/".format(sys.argv[1])

data = {}
keys = [ str(uuid.uuid4())[24:-1] for x in range(10) ]
for k in keys:
    with open('data.json', 'r') as f:
        data = json.loads(f.readline())
        while data:
            key = str(data["id"])
            try:
                requests.post(url=URL+k+"/"+key, json=data)
            except Exception as e:
                print(e)
            data = f.readline()
            if not data:
                break
            data = json.loads(data)
    
