#!/usr/bin/python

from shutil import which
import subprocess
import threading
import argparse
import ctypes
import yaml
import os
import re

class bcolors:
    HEADER = '\033[95m'
    OKBLUE = '\033[94m'
    OKGREEN = '\033[92m'
    WARNING = '\033[93m'
    FAIL = '\033[91m'
    ENDC = '\033[0m'
    BOLD = '\033[1m'
    UNDERLINE = '\033[4m'

class Pod:
    def __init__(self, name, port):
        self.name = name
        self.port = port

def error(msg):
    print(f"{bcolors.FAIL}[ERRO]{bcolors.ENDC}: {msg}")

def warning(msg):
    print(f"{bcolors.WARNING}[WARN]{bcolors.ENDC}: {msg}")

def info(msg):
    print(f"{bcolors.OKBLUE}[INFO]{bcolors.ENDC}: {msg}")

def ok(msg):
    print(f"{bcolors.OKGREEN}[ OK ]{bcolors.ENDC}: {msg}")

def str_to_bool(v):
    if isinstance(v, bool):
       return v
    if v.lower() in ('yes', 'true', 't', 'y', '1'):
        return True
    elif v.lower() in ('no', 'false', 'f', 'n', '0'):
        return False
    else:
        raise argparse.ArgumentTypeError('Boolean value expected.')

def has_kubectl():
    return which("kubectl") != None

def file_exists(path):
    return os.path.isfile(path)

def get_kuber_pods_text():
    info(f"Get kuber pods ")
    out = subprocess.Popen([f"kubectl get pods"],
           stdout=subprocess.PIPE, 
           stderr=subprocess.PIPE,
           shell=True,
          )

    stdout, stderr = out.communicate()
    exit_code = out.wait()
    if exit_code != 0:
        error(stderr.decode("utf-8"))
        return None

    pods_text = stdout.decode("utf-8").splitlines()[1:]

    info(f"Kuber pods:\n{stdout.decode('utf-8')}")
    return pods_text



def get_pod_name_by_regexp(pods_text, regexp):
    for line in pods_text:
        if not regexp.match(line):
           continue

        return line.split()[0]

    return None

def get_pods(pods_text, pods_file):
    with open(f"{pods_file}") as f:
        pods = []
        pods_yaml = yaml.load(f, Loader=yaml.FullLoader)
        for pod_yaml in pods_yaml:
            expr = None
            port = None
            for k, v in pod_yaml.items():
                if k == "expr":
                    expr = v
                elif k == "port":
                    port = v

            if expr == None or port == None:
                error("Invalid pods config schema")
                return None

            regexp = re.compile(expr)
            name = get_pod_name_by_regexp(pods_text, regexp)

            if name == None:
                error(f"Pod by regexp ({regexp}) not found")
                return None

            pods.append(Pod(name, port))

        return pods

    return None

def run_kubectl(pod, port, auto_restart):
    exit_code = 0
    while True:
        info(f"Start kubectl: pod={pod}, port={port}, auto_restart={auto_restart}")
        process = subprocess.Popen([
            f"kubectl port-forward {pod} {port}"
            ], 
           stdout=subprocess.PIPE, 
           stderr=subprocess.PIPE,
           shell=True,
           )
        _, stderr = process.communicate()
        exit_code = process.wait()
        if exit_code != 0:
            error(stderr.decode("utf-8"))
            error(f"Kubectl pod={pod} exited and return code: {exit_code}")
        else:
            warning(f"Kubectl pod={pod} exited and return code: {exit_code}")

        if not auto_restart:
            info(f"Stop kubectl: pod={pod}, port={port}, auto_restart={auto_restart}")
            break


def get_kubectl_thread(pod, port, auto_restart):
    return threading.Thread(target=run_kubectl, name=pod, args=(pod, port, auto_restart,))

def start_threads(threads):
    for _, t in enumerate(threads):
        t.daemon = True
        t.start()

def join_threads(threads):
    for _, t in enumerate(threads):
        t.join()

def main():
    # CLI setting up
    parser = argparse.ArgumentParser(description="Starting up all needed kuber instances")
    parser.add_argument("-p", "--pods", nargs=1, type=str, metavar="KUBER_PODS", default=[os.environ.get("KUBER_PODS")], help="Path to kuber pods file. May be install by environment variable $KUBER_PODS")
    parser.add_argument("-a", "--auto-restart", nargs=1, type=str_to_bool, default=[True], metavar="[true, false]", help="Enable or disable auto restart kuber instance if it exit")
    args = parser.parse_args()


    pods_file = args.pods[0]
    auto_restart = args.auto_restart[0]

    if not has_kubectl():
        parser.print_help()
        error("'kubectl' not found")
        return 1

    if pods_file == None:
         parser.print_help()
         error("Pods param is None")
         return 1

    if not file_exists(pods_file):
         parser.print_help()
         error("Pods not found")
         return 1

    try:    
        pods_text = get_kuber_pods_text()
        if pods_text == None:
            return 1

        pods = get_pods(pods_text, pods_file)
        if pods == None:
            return 1

        kubectl_instances = []
        for pod in pods:
            kubectl_instances.append(get_kubectl_thread(pod.name, pod.port, auto_restart))

    except KeyboardInterrupt:
        error("Keyboard interrupt was catched. Process aborted")
        return 1

    try:
        start_threads(kubectl_instances)
        join_threads(kubectl_instances)
    except KeyboardInterrupt:
        ok("Keyboard interrupt was catched. Kubectl instances was stoped")
        return 0

    ok("All kubectl instances was exited")


if __name__ == "__main__":
    main()