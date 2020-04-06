#!/usr/bin/env python3

import requests
import json
import string
import random
import sys

URL = "http://localhost:9000/api/v1/kv/"

data = {}
iterations = 0
while iterations <= 10:
    with open('data.json', 'r') as f:
        data = json.loads(f.readline())
        while data:
            key = ''.join([random.choice(string.ascii_letters + string.digits) for n in range(8)])
            buckets = []
            for n in range(random.randint(1,10)):
                buckets.append(''.join([random.choice(string.ascii_letters + string.digits) for n in range(32)]))
            path = '/'.join(buckets)
            try:
                requests.post(url=URL+path+'/'+key, json=data)
            except Exception as e:
                print(e)
            data = f.readline()
            if not data:
                break
            data = json.loads(data) 
    iterations += 1

    
