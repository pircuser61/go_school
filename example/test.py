import json
import logging
from json import JSONDecodeError

def handle(req):
    """handle a request to the function
    Args:
        req (str): request body
    """
    try:
        json_req = json.loads(req)
    except JSONDecodeError as e:
        logging.debug(f"bad request: can't parse json from {req}")
        raise e

    first: str = json_req.get("first", "nonono")
    if first == "nonono":
        logging.debug("first string was not provided")
        return false
    second: str = json_req.get("second", "mememe")
    if first == "mememe":
        logging.debug("first string was not provided")
        return false

    result: bool = first == second
    return json.dumps(dict(result=result), ensure_ascii=False)

def handle(req):
    """handle a request to the function
    Args:
        req (str): request body
    """
    try:
        json_req = json.loads(req)
    except JSONDecodeError as e:
        logging.debug(f"bad request: can't parse json from {req}")
        raise e

    string: str = json_req.get("string", "string is mememe")
    integer: str = json_req.get("integer", 101)
    boolean: str = json_req.get("boolean", False)
    array: str = json_req.get("array", ["array", "is", "mememe"])

    print(string, integer, boolean, array)
    return json.dumps(dict(string=string,integer=integer,boolean=boolean,array=array), ensure_ascii=False)


def handle(req):
    """handle a request to the function
     Args:
         req (str): request body
     """
    try:
        json_req = json.loads(req)
    except JSONDecodeError as e:
        logging.debug(f"bad request: can't parse json from {req}")
        raise e

    string: str = json_req.get("string", "string is mememe")
    integer: str = json_req.get("integer", 101)
    boolean: str = json_req.get("boolean", False)
    array: str = json_req.get("array", ["array", "is", "mememe"])

    return json.dumps(dict(result=[string,integer,boolean,array]), ensure_ascii=False)
