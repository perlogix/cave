#!/usr/bin/env python3

import requests
import json
import string
import random
import sys

URL = "http://{}/api/v1/kv/".format(sys.argv[1])

data = {}
iterations = 0
while iterations <= 10:
    with open('data.json', 'r') as f:
        data = json.loads(f.readline())
        while data:
            key = str(data["id"])
            try:
                requests.post(url=URL+str(iterations)+"/"+key, json=data)
            except Exception as e:
                print(e)
            data = f.readline()
            if not data:
                break
            data = json.loads(data) 
    iterations += 1

    
