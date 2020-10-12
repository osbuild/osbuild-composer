"""
Wrapper around openssl providing several methods to manage X.509 certificates.
"""

import contextlib
import subprocess
import tempfile
from os import path


def _config(common_name, subject_alternative_names):
    """
    Returns config for creating x.509 v3 certificate using openssl.
    :param common_name: Common name of the subject owning the certificate
    :param subject_alternative_names: Subject alternative names of the subject owning the certificate
    :return: openssl config as a string
    """
    alt_names = "\n".join((f"DNS.{i + 1} = {san}" for (i, san) in enumerate(subject_alternative_names)))
    return f"""
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = {common_name}

[v3_req]
keyUsage = critical,keyEncipherment, dataEncipherment, digitalSignature
extendedKeyUsage = critical,serverAuth,clientAuth,emailProtection
basicConstraints = critical,CA:FALSE
subjectAltName = @alt_names

[alt_names]
{alt_names}
"""


@contextlib.contextmanager
def _config_file(common_name, subject_alternative_names):
    """
    Provides a context manager for a temporary openssl config created by _config()
    :param common_name: Common name of the subject owning the certificate
    :param subject_alternative_names: Subject alternative names of the subject owning the certificate
    """
    with tempfile.TemporaryDirectory() as dir:
        filename = path.join(dir, "openssl.conf")
        f = open(filename, "w")
        f.write(_config(common_name, subject_alternative_names))
        f.close()
        yield filename


def generate_csr(out, keyout, common_name, subject_alternative_names):
    """
    Generates an X.509 v3 certificate signing request with subject alternative names
    :param out: Location where the new certificate should be stored
    :param keyout: Location where the private key belonging to the new csr should be stored
    :param common_name: Common name of the subject requesting the certificate.
    :param subject_alternative_names: One or more subject alternative names of the subject requesting the certificate.
    """
    with _config_file(common_name, subject_alternative_names) as config:
        subprocess.check_call([
            "openssl", "req", "-new", "-nodes",
            "-config", config,
            "-keyout", keyout,
            "-out", out
        ])


def sign_csr(csr, out, ca, ca_key, common_name, subject_alternative_names):
    """
    Signs an X.509 v3 certificate.
    :param csr: Location of the certificate signing request.
    :param out: Location where the new certificate should be stored.
    :param ca: Certificate of the CA used to sign the request.
    :param ca_key: Private key of the CA used to sign the request.
    :param common_name: Common name of the subject owning the new certificate.
    :param subject_alternative_names: One or more subject alternative names of the subject owning the new certificate.
    :return:
    """
    with _config_file(common_name, subject_alternative_names) as config:
        subprocess.check_call([
            "openssl", "x509", "-req", "-CAcreateserial",
            "-extfile", config,
            "-extensions", "v3_req",
            "-in", csr,
            "-CA", ca,
            "-CAkey", ca_key,
            "-out", out
        ])


def generate_certificate(out, keyout, ca, ca_key, common_name, subject_alternative_names):
    """
    Generates an X.509 v3 certificate and a private key belonging to it.
    :param out: Location where the new certificate should be stored.
    :param keyout: Location where the private key belonging to the new certificate should be stored.
    :param ca: Certificate of the CA used to sign the new certificate.
    :param ca_key: Private key of the CA used to sign the new certificate.
    :param common_name: Common name of the subject owning the new certificate.
    :param subject_alternative_names: One or more subject alternative names of the subject owning the new certificate.
    """
    with tempfile.TemporaryDirectory() as dir:
        csr = path.join(dir, "csr.pem")
        generate_csr(csr, keyout, common_name, subject_alternative_names)
        sign_csr(csr, out, ca, ca_key, common_name, subject_alternative_names)


def generate_self_signed_certificate(out, keyout, common_name, subject_alternative_names):
    """
    Generates a self-signed X.509 v3 certificate and a private key belonging to it.
    :param out: Location where the new certificate should be stored.
    :param keyout: Location where the private key belonging to the new certificate should be stored.
    :param common_name: Common name of the subject owning the new certificate.
    :param subject_alternative_names: One or more subject alternative names of the subject owning the new certificate.
    """
    with _config_file(common_name, subject_alternative_names) as config:
        subprocess.check_call([
            "openssl", "req", "-nodes", "-x509",
            "-config", config,
            "-keyout", keyout,
            "-out", out
        ])
