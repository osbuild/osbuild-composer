#!/usr/bin/env python3
import argparse, subprocess

def launch_server(address, port, certdir):
  cmd = [
      "/usr/libexec/osbuild-composer/osbuild-mock-openid-provider",
      "-a", str.join(":", [address, port]),
      "-rsaPubPem", f"{certdir}/client-crt.pem",
      "-rsaPem", f"{certdir}/client-key.pem",
  ]
  print("Running oath server")
  return subprocess.run(
      cmd,
      cwd="/usr/libexec/osbuild-composer",
      stdin=subprocess.DEVNULL,
      stderr=subprocess.STDOUT,
  )

def main():
  parser = argparse.ArgumentParser()
  parser.add_argument("-a", "--address", help="IP address for the server", type=str, default="localhost")
  parser.add_argument("-p", "--port", help="Port for the server", type=str, default="8080")
  parser.add_argument("-c", "--certdir", help="The location dir of the certs", type=str, default="/etc/osbuild-composer")
  args = parser.parse_args()
  launch_server(args.address, args.port, args.certdir)

if __name__ == "__main__":
  main()
