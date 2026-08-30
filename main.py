import boto3

s3 = boto3.client(
    "s3",
    endpoint_url="http://localhost:8333",
    aws_access_key_id="any",
    aws_secret_access_key="any",
)

s3.create_bucket(Bucket="mybucket")
s3.upload_file("myfile.txt", "mybucket", "myfile.txt")

# Download
s3.download_file("mybucket", "myfile.txt", "downloaded_file.txt")

# import requests

# with open("myfile.txt", "rb") as f:
#     requests.post("http://localhost:8888/uploads/myfile.txt", files={"file": f})

# r = requests.get("http://localhost:8888/uploads/myfile.txt")
# print(r.content)

# -- https://github.com/ginuerzh/weedo: seaweedfs client