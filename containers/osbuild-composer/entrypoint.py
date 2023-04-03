"""entrypoint - Containerized OSBuild Composer

This provides the entrypoint for a containerized osbuild-composer image. It
spawns `osbuild-composer` on start and manages it until it exits. The main
purpose of this entrypoint is to prepare everything to be usable from within
a container.
"""

import argparse
import contextlib
import os
import pathlib
import signal
import socket
import subprocess
import sys
import time


class Cli(contextlib.AbstractContextManager):
    """Command Line Interface"""

    SYSLOG_RETRIES=5
    SYSLOG_WAIT=2.0

    def __init__(self, argv):
        self.args = None
        self._argv = argv
        self._exitstack = None
        self._parser = None

    def _parse_args(self):
        self._parser = argparse.ArgumentParser(
            add_help=True,
            allow_abbrev=False,
            argument_default=None,
            description="Containerized OSBuild Composer",
            prog="container/osbuild-composer",
        )

        self._parser.add_argument(
            "--shutdown-wait-period",
            type=int,
            default=0,
            dest="shutdown_wait_period",
            help="Wait period in seconds before terminating child processes",
        )

        # --[no-]composer-api
        self._parser.add_argument(
            "--composer-api",
            action="store_true",
            dest="composer_api",
            help="Enable the composer-API",
        )
        self._parser.add_argument(
            "--no-composer-api",
            action="store_false",
            dest="composer_api",
            help="Disable the composer-API",
        )
        self._parser.add_argument(
            "--composer-api-port",
            type=int,
            default=8080,
            dest="composer_api_port",
            help="Port which the composer-API listens on",
        )
        self._parser.add_argument(
            "--composer-api-bind-address",
            type=str,
            default="::",
            dest="composer_api_bind_address",
            help="Bind the composer API to the specified address",
        )

        # --[no-]local-worker-api
        self._parser.add_argument(
            "--local-worker-api",
            action="store_true",
            dest="local_worker_api",
            help="Enable the local-worker-API",
        )
        self._parser.add_argument(
            "--no-local-worker-api",
            action="store_false",
            dest="local_worker_api",
            help="Disable the local-worker-API",
        )

        # --[no-]remote-worker-api
        self._parser.add_argument(
            "--remote-worker-api",
            action="store_true",
            dest="remote_worker_api",
            help="Enable the remote-worker-API",
        )
        self._parser.add_argument(
            "--no-remote-worker-api",
            action="store_false",
            dest="remote_worker_api",
            help="Disable the remote-worker-API",
        )
        self._parser.add_argument(
            "--remote-worker-api-port",
            type=int,
            default=8700,
            dest="remote_worker_api_port",
            help="Port which the remote-worker API listens on",
        )
        self._parser.add_argument(
            "--remote-worker-api-bind-address",
            type=str,
            default="::",
            dest="remote_worker_api_bind_address",
            help="Bind the remote worker API to the specified address",
        )

        # --[no-]weldr-api
        self._parser.add_argument(
            "--weldr-api",
            action="store_true",
            dest="weldr_api",
            help="Enable the weldr-API",
        )
        self._parser.add_argument(
            "--no-weldr-api",
            action="store_false",
            dest="weldr_api",
            help="Disable the weldr-API",
        )

        self._parser.set_defaults(
            builtin_worker=False,
            composer_api=False,
            local_worker_api=False,
            remote_worker_api=False,
            weldr_api=False,
        )

        return self._parser.parse_args(self._argv[1:])

    def __enter__(self):
        self._exitstack = contextlib.ExitStack()
        self.args = self._parse_args()
        return self

    def __exit__(self, exc_type, exc_value, exc_tb):
        self._exitstack.close()
        self._exitstack = None

    def _prepare_sockets(self):
        # Prepare all the API sockets that osbuild-composer expectes, and make
        # sure to pass them according to the systemd socket-activation API.
        #
        # Note that we rely on this being called early, so we get the correct
        # FD numbers assigned. We need FD-#3 onwards for compatibility with
        # socket activation (because python `subprocess.Popen` does not support
        # renumbering the sockets we pass down).

        index = 3
        sockets = []
        names = []

        # osbuild-composer.socket
        if self.args.weldr_api:
            print("Create weldr-api socket", file=sys.stderr)
            sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            self._exitstack.enter_context(contextlib.closing(sock))
            sock.bind("/run/weldr/api.socket")
            sock.listen()
            sockets.append(sock)
            names.append("osbuild-composer.socket")

            assert(sock.fileno() == index)
            index += 1

        # osbuild-composer-api.socket
        if self.args.composer_api:
            print("Create composer-api socket on port {}".format(self.args.composer_api_port) , file=sys.stderr)
            sock = socket.socket(socket.AF_INET6, socket.SOCK_STREAM)
            self._exitstack.enter_context(contextlib.closing(sock))
            sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            sock.setsockopt(socket.IPPROTO_IPV6, socket.IPV6_V6ONLY, 0)
            sock.bind((self.args.composer_api_bind_address, self.args.composer_api_port))
            sock.listen()
            sockets.append(sock)
            names.append("osbuild-composer-api.socket")

            assert(sock.fileno() == index)
            index += 1

        # osbuild-local-worker.socket
        if self.args.local_worker_api:
            print("Create local-worker-api socket", file=sys.stderr)
            sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            self._exitstack.enter_context(contextlib.closing(sock))
            sock.bind("/run/osbuild-composer/job.socket")
            sock.listen()
            sockets.append(sock)
            names.append("osbuild-local-worker.socket")

            assert(sock.fileno() == index)
            index += 1

        # osbuild-remote-worker.socket
        if self.args.remote_worker_api:
            print(f"Create remote-worker-api socket on address [{self.args.remote_worker_api_bind_address}]:{self.args.remote_worker_api_port}", file=sys.stderr)
            sock = socket.socket(socket.AF_INET6, socket.SOCK_STREAM)
            self._exitstack.enter_context(contextlib.closing(sock))
            sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            sock.setsockopt(socket.IPPROTO_IPV6, socket.IPV6_V6ONLY, 0)
            sock.bind((self.args.remote_worker_api_bind_address, self.args.remote_worker_api_port))
            sock.listen(256)
            sockets.append(sock)
            names.append("osbuild-remote-worker.socket")

            assert(sock.fileno() == index)
            index += 1

        # Prepare FD environment for the child process.
        os.environ["LISTEN_FDS"] = str(len(sockets))
        os.environ["LISTEN_FDNAMES"] = ":".join(names)

        return sockets

    @staticmethod
    def _spawn_worker():
        cmd = [
            "/usr/libexec/osbuild-composer/osbuild-worker",
            "-unix",
            "/run/osbuild-composer/job.socket",
        ]

        env = os.environ.copy()
        env["CACHE_DIRECTORY"] = "/var/cache/osbuild-worker"
        env["STATE_DIRECTORY"] = "/var/lib/osbuild-worker"

        return subprocess.Popen(
            cmd,
            cwd="/",
            env=env,
            stdin=subprocess.DEVNULL,
            stderr=subprocess.STDOUT,
        )

    @staticmethod
    def _wait_for_syslog(addr):
        print("Waiting for fluentd syslog server", file=sys.stderr)
        for _ in range(Cli.SYSLOG_RETRIES):
            sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            try:
                sock.connect(addr)
            except ConnectionRefusedError:
                time.sleep(Cli.SYSLOG_WAIT)
                continue
            except (NameError, ConnectionError) as ex:
                print("Unexpected error:", ex , file=sys.stderr)
                break
            finally:
                sock.close()

            print("fluentd syslog server is up", file=sys.stderr)
            break

    @staticmethod
    def _spawn_composer(sockets):
        cmd = [
            "/usr/libexec/osbuild-composer/osbuild-composer",
            "-verbose",
        ]

        # Prepare the environment for osbuild-composer. Note that we cannot use
        # the `env` parameter of `subprocess.Popen()`, because it conflicts
        # with the `preexec_fn=` parameter. Therefore, we have to modify the
        # caller's environment.
        os.environ["CACHE_DIRECTORY"] = "/var/cache/osbuild-composer"
        os.environ["STATE_DIRECTORY"] = "/var/lib/osbuild-composer"

        # We need to set `LISTEN_PID=` to the target PID. The only way python
        # allows us to do this is to hook into `preexec_fn=`, which is executed
        # by `subprocess.Popen()` after forking, but before executing the new
        # executable.
        preexec_setenv = lambda: os.putenv("LISTEN_PID", str(os.getpid()))

        return subprocess.Popen(
            cmd,
            cwd="/usr/libexec/osbuild-composer",
            stdin=subprocess.DEVNULL,
            stderr=subprocess.STDOUT,
            pass_fds=[sock.fileno() for sock in sockets],
            preexec_fn=preexec_setenv,
        )

    def run(self):
        """Program Runtime"""

        proc_composer = None
        proc_worker = None
        res = 0
        sockets = self._prepare_sockets()

        def handler(signum, frame):
            if self.args.shutdown_wait_period:
                time.sleep(self.args.shutdown_wait_period)
            proc_composer.terminate()
            proc_worker.terminate()

        signal.signal(signal.SIGTERM, handler)

        liveness = pathlib.Path('/tmp/osbuild-composer-live')

        liveness.touch()

        try:
            if self.args.builtin_worker:
                proc_worker = self._spawn_worker()

            if any([self.args.weldr_api, self.args.composer_api, self.args.local_worker_api, self.args.remote_worker_api]):
                if "SYSLOG_SERVER" in os.environ:
                    addr, port = os.environ["SYSLOG_SERVER"].split(":")
                    self._wait_for_syslog((addr, int(port)))
                proc_composer = self._spawn_composer(sockets)

            if proc_composer:
                res = proc_composer.wait()

            if proc_worker:
                if proc_composer:
                    proc_worker.terminate()
                proc_worker.wait()

        except KeyboardInterrupt:
            if proc_composer:
                proc_composer.terminate()
                res = proc_composer.wait()
            if proc_worker:
                proc_worker.terminate()
                proc_worker.wait()
        except:
            if proc_worker:
                proc_worker.kill()
            if proc_composer:
                proc_composer.kill()
            raise
        finally:
            liveness.unlink()

        return res


if __name__ == "__main__":
    with Cli(sys.argv) as global_main:
        sys.exit(global_main.run())
