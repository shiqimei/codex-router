from routing import Provider
from http.server import ThreadingHTTPServer
Provider.delay=30
server=ThreadingHTTPServer(('127.0.0.1',48125),Provider)
print('Desktop fixture: http://127.0.0.1:48125/v1',flush=True)
server.serve_forever()
