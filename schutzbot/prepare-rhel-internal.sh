#!/bin/bash
set -euo pipefail

# Colorful output.
function greenprint {
  echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

ALL_ARCHES="aarch64 ppc64le s390x x86_64"

if [ -e ../tools/define-compose-url.sh ]
then
    source ../tools/define-compose-url.sh
else
    source ./tools/define-compose-url.sh
fi

# Create a repository file for installing the osbuild-composer RPMs
greenprint "📜 Generating dnf repository file"
rm -f rhel"${VERSION_ID%.*}"internal.repo
for ARCH in $ALL_ARCHES; do
    tee -a rhel"${VERSION_ID%.*}"internal.repo << EOF

[rhel${VERSION_ID}-internal-baseos-${ARCH}]
name=RHEL Internal BaseOS
baseurl=${COMPOSE_URL}/compose/BaseOS/${ARCH}/os/
enabled=1
gpgcheck=0
# Default dnf repo priority is 99. Lower number means higher priority.
priority=1

[rhel${VERSION_ID}-internal-appstream-${ARCH}]
name=RHEL Internal AppStream
baseurl=${COMPOSE_URL}/compose/AppStream/${ARCH}/os/
enabled=1
gpgcheck=0
# osbuild-composer repo priority is 5
priority=1
EOF
done


# Create an osbuild-composer JSON file for content consumption
greenprint "📜 Generate osbuild-composer JSON source file"
tee rhel-"${VERSION_ID%.*}".json << EOF
{
EOF

for ARCH in $ALL_ARCHES; do
    tee -a rhel-"${VERSION_ID%.*}".json << EOF
    "${ARCH}": [
        {
            "name": "baseos-${ARCH}",
            "baseurl": "${COMPOSE_URL}/compose/BaseOS/${ARCH}/os/",
            "gpgkey": "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBErgSTsBEACh2A4b0O9t+vzC9VrVtL1AKvUWi9OPCjkvR7Xd8DtJxeeMZ5eF\n0HtzIG58qDRybwUe89FZprB1ffuUKzdE+HcL3FbNWSSOXVjZIersdXyH3NvnLLLF\n0DNRB2ix3bXG9Rh/RXpFsNxDp2CEMdUvbYCzE79K1EnUTVh1L0Of023FtPSZXX0c\nu7Pb5DI5lX5YeoXO6RoodrIGYJsVBQWnrWw4xNTconUfNPk0EGZtEnzvH2zyPoJh\nXGF+Ncu9XwbalnYde10OCvSWAZ5zTCpoLMTvQjWpbCdWXJzCm6G+/hx9upke546H\n5IjtYm4dTIVTnc3wvDiODgBKRzOl9rEOCIgOuGtDxRxcQkjrC+xvg5Vkqn7vBUyW\n9pHedOU+PoF3DGOM+dqv+eNKBvh9YF9ugFAQBkcG7viZgvGEMGGUpzNgN7XnS1gj\n/DPo9mZESOYnKceve2tIC87p2hqjrxOHuI7fkZYeNIcAoa83rBltFXaBDYhWAKS1\nPcXS1/7JzP0ky7d0L6Xbu/If5kqWQpKwUInXtySRkuraVfuK3Bpa+X1XecWi24JY\nHVtlNX025xx1ewVzGNCTlWn1skQN2OOoQTV4C8/qFpTW6DTWYurd4+fE0OJFJZQF\nbuhfXYwmRlVOgN5i77NTIJZJQfYFj38c/Iv5vZBPokO6mffrOTv3MHWVgQARAQAB\ntDNSZWQgSGF0LCBJbmMuIChyZWxlYXNlIGtleSAyKSA8c2VjdXJpdHlAcmVkaGF0\nLmNvbT6JAjYEEwECACAFAkrgSTsCGwMGCwkIBwMCBBUCCAMEFgIDAQIeAQIXgAAK\nCRAZni+R/UMdUWzpD/9s5SFR/ZF3yjY5VLUFLMXIKUztNN3oc45fyLdTI3+UClKC\n2tEruzYjqNHhqAEXa2sN1fMrsuKec61Ll2NfvJjkLKDvgVIh7kM7aslNYVOP6BTf\nC/JJ7/ufz3UZmyViH/WDl+AYdgk3JqCIO5w5ryrC9IyBzYv2m0HqYbWfphY3uHw5\nun3ndLJcu8+BGP5F+ONQEGl+DRH58Il9Jp3HwbRa7dvkPgEhfFR+1hI+Btta2C7E\n0/2NKzCxZw7Lx3PBRcU92YKyaEihfy/aQKZCAuyfKiMvsmzs+4poIX7I9NQCJpyE\nIGfINoZ7VxqHwRn/d5mw2MZTJjbzSf+Um9YJyA0iEEyD6qjriWQRbuxpQXmlAJbh\n8okZ4gbVFv1F8MzK+4R8VvWJ0XxgtikSo72fHjwha7MAjqFnOq6eo6fEC/75g3NL\nGht5VdpGuHk0vbdENHMC8wS99e5qXGNDued3hlTavDMlEAHl34q2H9nakTGRF5Ki\nJUfNh3DVRGhg8cMIti21njiRh7gyFI2OccATY7bBSr79JhuNwelHuxLrCFpY7V25\nOFktl15jZJaMxuQBqYdBgSay2G0U6D1+7VsWufpzd/Abx1/c3oi9ZaJvW22kAggq\ndzdA27UUYjWvx42w9menJwh/0jeQcTecIUd0d0rFcw/c1pvgMMl/Q73yzKgKYw==\n=zbHE\n-----END PGP PUBLIC KEY BLOCK-----\n-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBFsy23UBEACUKSphFEIEvNpy68VeW4Dt6qv+mU6am9a2AAl10JANLj1oqWX+\noYk3en1S6cVe2qehSL5DGVa3HMUZkP3dtbD4SgzXzxPodebPcr4+0QNWigkUisri\nXGL5SCEcOP30zDhZvg+4mpO2jMi7Kc1DLPzBBkgppcX91wa0L1pQzBcvYMPyV/Dh\nKbQHR75WdkP6OA2JXdfC94nxYq+2e0iPqC1hCP3Elh+YnSkOkrawDPmoB1g4+ft/\nxsiVGVy/W0ekXmgvYEHt6si6Y8NwXgnTMqxeSXQ9YUgVIbTpsxHQKGy76T5lMlWX\n4LCOmEVomBJg1SqF6yi9Vu8TeNThaDqT4/DddYInd0OO69s0kGIXalVgGYiW2HOD\nx2q5R1VGCoJxXomz+EbOXY+HpKPOHAjU0DB9MxbU3S248LQ69nIB5uxysy0PSco1\nsdZ8sxRNQ9Dw6on0Nowx5m6Thefzs5iK3dnPGBqHTT43DHbnWc2scjQFG+eZhe98\nEll/kb6vpBoY4bG9/wCG9qu7jj9Z+BceCNKeHllbezVLCU/Hswivr7h2dnaEFvPD\nO4GqiWiwOF06XaBMVgxA8p2HRw0KtXqOpZk+o+sUvdPjsBw42BB96A1yFX4jgFNA\nPyZYnEUdP6OOv9HSjnl7k/iEkvHq/jGYMMojixlvXpGXhnt5jNyc4GSUJQARAQAB\ntDNSZWQgSGF0LCBJbmMuIChhdXhpbGlhcnkga2V5KSA8c2VjdXJpdHlAcmVkaGF0\nLmNvbT6JAjkEEwECACMFAlsy23UCGwMHCwkIBwMCAQYVCAIJCgsEFgIDAQIeAQIX\ngAAKCRD3b2bD1AgnknqOD/9fB2ASuG2aJIiap4kK58R+RmOVM4qgclAnaG57+vjI\nnKvyfV3NH/keplGNRxwqHekfPCqvkpABwhdGEXIE8ILqnPewIMr6PZNZWNJynZ9i\neSMzVuCG7jDoGyQ5/6B0f6xeBtTeBDiRl7+Alehet1twuGL1BJUYG0QuLgcEzkaE\n/gkuumeVcazLzz7L12D22nMk66GxmgXfqS5zcbqOAuZwaA6VgSEgFdV2X2JU79zS\nBQJXv7NKc+nDXFG7M7EHjY3Rma3HXkDbkT8bzh9tJV7Z7TlpT829pStWQyoxKCVq\nsEX8WsSapTKA3P9YkYCwLShgZu4HKRFvHMaIasSIZWzLu+RZH/4yyHOhj0QB7XMY\neHQ6fGSbtJ+K6SrpHOOsKQNAJ0hVbSrnA1cr5+2SDfel1RfYt0W9FA6DoH/S5gAR\ndzT1u44QVwwp3U+eFpHphFy//uzxNMtCjjdkpzhYYhOCLNkDrlRPb+bcoL/6ePSr\n016PA7eEnuC305YU1Ml2WcCn7wQV8x90o33klJmEkWtXh3X39vYtI4nCPIvZn1eP\nVy+F+wWt4vN2b8oOdlzc2paOembbCo2B+Wapv5Y9peBvlbsDSgqtJABfK8KQq/jK\nYl3h5elIa1I3uNfczeHOnf1enLOUOlq630yeM/yHizz99G1g+z/guMh5+x/OHraW\niA==\n=+Gxh\n-----END PGP PUBLIC KEY BLOCK-----\n"
        },
        {
            "name": "appstream-${ARCH}",
            "baseurl": "${COMPOSE_URL}/compose/AppStream/${ARCH}/os/",
            "gpgkey": "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBErgSTsBEACh2A4b0O9t+vzC9VrVtL1AKvUWi9OPCjkvR7Xd8DtJxeeMZ5eF\n0HtzIG58qDRybwUe89FZprB1ffuUKzdE+HcL3FbNWSSOXVjZIersdXyH3NvnLLLF\n0DNRB2ix3bXG9Rh/RXpFsNxDp2CEMdUvbYCzE79K1EnUTVh1L0Of023FtPSZXX0c\nu7Pb5DI5lX5YeoXO6RoodrIGYJsVBQWnrWw4xNTconUfNPk0EGZtEnzvH2zyPoJh\nXGF+Ncu9XwbalnYde10OCvSWAZ5zTCpoLMTvQjWpbCdWXJzCm6G+/hx9upke546H\n5IjtYm4dTIVTnc3wvDiODgBKRzOl9rEOCIgOuGtDxRxcQkjrC+xvg5Vkqn7vBUyW\n9pHedOU+PoF3DGOM+dqv+eNKBvh9YF9ugFAQBkcG7viZgvGEMGGUpzNgN7XnS1gj\n/DPo9mZESOYnKceve2tIC87p2hqjrxOHuI7fkZYeNIcAoa83rBltFXaBDYhWAKS1\nPcXS1/7JzP0ky7d0L6Xbu/If5kqWQpKwUInXtySRkuraVfuK3Bpa+X1XecWi24JY\nHVtlNX025xx1ewVzGNCTlWn1skQN2OOoQTV4C8/qFpTW6DTWYurd4+fE0OJFJZQF\nbuhfXYwmRlVOgN5i77NTIJZJQfYFj38c/Iv5vZBPokO6mffrOTv3MHWVgQARAQAB\ntDNSZWQgSGF0LCBJbmMuIChyZWxlYXNlIGtleSAyKSA8c2VjdXJpdHlAcmVkaGF0\nLmNvbT6JAjYEEwECACAFAkrgSTsCGwMGCwkIBwMCBBUCCAMEFgIDAQIeAQIXgAAK\nCRAZni+R/UMdUWzpD/9s5SFR/ZF3yjY5VLUFLMXIKUztNN3oc45fyLdTI3+UClKC\n2tEruzYjqNHhqAEXa2sN1fMrsuKec61Ll2NfvJjkLKDvgVIh7kM7aslNYVOP6BTf\nC/JJ7/ufz3UZmyViH/WDl+AYdgk3JqCIO5w5ryrC9IyBzYv2m0HqYbWfphY3uHw5\nun3ndLJcu8+BGP5F+ONQEGl+DRH58Il9Jp3HwbRa7dvkPgEhfFR+1hI+Btta2C7E\n0/2NKzCxZw7Lx3PBRcU92YKyaEihfy/aQKZCAuyfKiMvsmzs+4poIX7I9NQCJpyE\nIGfINoZ7VxqHwRn/d5mw2MZTJjbzSf+Um9YJyA0iEEyD6qjriWQRbuxpQXmlAJbh\n8okZ4gbVFv1F8MzK+4R8VvWJ0XxgtikSo72fHjwha7MAjqFnOq6eo6fEC/75g3NL\nGht5VdpGuHk0vbdENHMC8wS99e5qXGNDued3hlTavDMlEAHl34q2H9nakTGRF5Ki\nJUfNh3DVRGhg8cMIti21njiRh7gyFI2OccATY7bBSr79JhuNwelHuxLrCFpY7V25\nOFktl15jZJaMxuQBqYdBgSay2G0U6D1+7VsWufpzd/Abx1/c3oi9ZaJvW22kAggq\ndzdA27UUYjWvx42w9menJwh/0jeQcTecIUd0d0rFcw/c1pvgMMl/Q73yzKgKYw==\n=zbHE\n-----END PGP PUBLIC KEY BLOCK-----\n-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBFsy23UBEACUKSphFEIEvNpy68VeW4Dt6qv+mU6am9a2AAl10JANLj1oqWX+\noYk3en1S6cVe2qehSL5DGVa3HMUZkP3dtbD4SgzXzxPodebPcr4+0QNWigkUisri\nXGL5SCEcOP30zDhZvg+4mpO2jMi7Kc1DLPzBBkgppcX91wa0L1pQzBcvYMPyV/Dh\nKbQHR75WdkP6OA2JXdfC94nxYq+2e0iPqC1hCP3Elh+YnSkOkrawDPmoB1g4+ft/\nxsiVGVy/W0ekXmgvYEHt6si6Y8NwXgnTMqxeSXQ9YUgVIbTpsxHQKGy76T5lMlWX\n4LCOmEVomBJg1SqF6yi9Vu8TeNThaDqT4/DddYInd0OO69s0kGIXalVgGYiW2HOD\nx2q5R1VGCoJxXomz+EbOXY+HpKPOHAjU0DB9MxbU3S248LQ69nIB5uxysy0PSco1\nsdZ8sxRNQ9Dw6on0Nowx5m6Thefzs5iK3dnPGBqHTT43DHbnWc2scjQFG+eZhe98\nEll/kb6vpBoY4bG9/wCG9qu7jj9Z+BceCNKeHllbezVLCU/Hswivr7h2dnaEFvPD\nO4GqiWiwOF06XaBMVgxA8p2HRw0KtXqOpZk+o+sUvdPjsBw42BB96A1yFX4jgFNA\nPyZYnEUdP6OOv9HSjnl7k/iEkvHq/jGYMMojixlvXpGXhnt5jNyc4GSUJQARAQAB\ntDNSZWQgSGF0LCBJbmMuIChhdXhpbGlhcnkga2V5KSA8c2VjdXJpdHlAcmVkaGF0\nLmNvbT6JAjkEEwECACMFAlsy23UCGwMHCwkIBwMCAQYVCAIJCgsEFgIDAQIeAQIX\ngAAKCRD3b2bD1AgnknqOD/9fB2ASuG2aJIiap4kK58R+RmOVM4qgclAnaG57+vjI\nnKvyfV3NH/keplGNRxwqHekfPCqvkpABwhdGEXIE8ILqnPewIMr6PZNZWNJynZ9i\neSMzVuCG7jDoGyQ5/6B0f6xeBtTeBDiRl7+Alehet1twuGL1BJUYG0QuLgcEzkaE\n/gkuumeVcazLzz7L12D22nMk66GxmgXfqS5zcbqOAuZwaA6VgSEgFdV2X2JU79zS\nBQJXv7NKc+nDXFG7M7EHjY3Rma3HXkDbkT8bzh9tJV7Z7TlpT829pStWQyoxKCVq\nsEX8WsSapTKA3P9YkYCwLShgZu4HKRFvHMaIasSIZWzLu+RZH/4yyHOhj0QB7XMY\neHQ6fGSbtJ+K6SrpHOOsKQNAJ0hVbSrnA1cr5+2SDfel1RfYt0W9FA6DoH/S5gAR\ndzT1u44QVwwp3U+eFpHphFy//uzxNMtCjjdkpzhYYhOCLNkDrlRPb+bcoL/6ePSr\n016PA7eEnuC305YU1Ml2WcCn7wQV8x90o33klJmEkWtXh3X39vYtI4nCPIvZn1eP\nVy+F+wWt4vN2b8oOdlzc2paOembbCo2B+Wapv5Y9peBvlbsDSgqtJABfK8KQq/jK\nYl3h5elIa1I3uNfczeHOnf1enLOUOlq630yeM/yHizz99G1g+z/guMh5+x/OHraW\niA==\n=+Gxh\n-----END PGP PUBLIC KEY BLOCK-----\n"
        }
EOF

    # only x86_64 has an RT repository
    if [ "$ARCH" == "x86_64" ]; then
        tee -a rhel-"${VERSION_ID%.*}".json << EOF
        ,{
            "name": "rt-${ARCH}",
            "baseurl": "${COMPOSE_URL}/compose/RT/${ARCH}/os/",
            "gpgkey": "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBErgSTsBEACh2A4b0O9t+vzC9VrVtL1AKvUWi9OPCjkvR7Xd8DtJxeeMZ5eF\n0HtzIG58qDRybwUe89FZprB1ffuUKzdE+HcL3FbNWSSOXVjZIersdXyH3NvnLLLF\n0DNRB2ix3bXG9Rh/RXpFsNxDp2CEMdUvbYCzE79K1EnUTVh1L0Of023FtPSZXX0c\nu7Pb5DI5lX5YeoXO6RoodrIGYJsVBQWnrWw4xNTconUfNPk0EGZtEnzvH2zyPoJh\nXGF+Ncu9XwbalnYde10OCvSWAZ5zTCpoLMTvQjWpbCdWXJzCm6G+/hx9upke546H\n5IjtYm4dTIVTnc3wvDiODgBKRzOl9rEOCIgOuGtDxRxcQkjrC+xvg5Vkqn7vBUyW\n9pHedOU+PoF3DGOM+dqv+eNKBvh9YF9ugFAQBkcG7viZgvGEMGGUpzNgN7XnS1gj\n/DPo9mZESOYnKceve2tIC87p2hqjrxOHuI7fkZYeNIcAoa83rBltFXaBDYhWAKS1\nPcXS1/7JzP0ky7d0L6Xbu/If5kqWQpKwUInXtySRkuraVfuK3Bpa+X1XecWi24JY\nHVtlNX025xx1ewVzGNCTlWn1skQN2OOoQTV4C8/qFpTW6DTWYurd4+fE0OJFJZQF\nbuhfXYwmRlVOgN5i77NTIJZJQfYFj38c/Iv5vZBPokO6mffrOTv3MHWVgQARAQAB\ntDNSZWQgSGF0LCBJbmMuIChyZWxlYXNlIGtleSAyKSA8c2VjdXJpdHlAcmVkaGF0\nLmNvbT6JAjYEEwECACAFAkrgSTsCGwMGCwkIBwMCBBUCCAMEFgIDAQIeAQIXgAAK\nCRAZni+R/UMdUWzpD/9s5SFR/ZF3yjY5VLUFLMXIKUztNN3oc45fyLdTI3+UClKC\n2tEruzYjqNHhqAEXa2sN1fMrsuKec61Ll2NfvJjkLKDvgVIh7kM7aslNYVOP6BTf\nC/JJ7/ufz3UZmyViH/WDl+AYdgk3JqCIO5w5ryrC9IyBzYv2m0HqYbWfphY3uHw5\nun3ndLJcu8+BGP5F+ONQEGl+DRH58Il9Jp3HwbRa7dvkPgEhfFR+1hI+Btta2C7E\n0/2NKzCxZw7Lx3PBRcU92YKyaEihfy/aQKZCAuyfKiMvsmzs+4poIX7I9NQCJpyE\nIGfINoZ7VxqHwRn/d5mw2MZTJjbzSf+Um9YJyA0iEEyD6qjriWQRbuxpQXmlAJbh\n8okZ4gbVFv1F8MzK+4R8VvWJ0XxgtikSo72fHjwha7MAjqFnOq6eo6fEC/75g3NL\nGht5VdpGuHk0vbdENHMC8wS99e5qXGNDued3hlTavDMlEAHl34q2H9nakTGRF5Ki\nJUfNh3DVRGhg8cMIti21njiRh7gyFI2OccATY7bBSr79JhuNwelHuxLrCFpY7V25\nOFktl15jZJaMxuQBqYdBgSay2G0U6D1+7VsWufpzd/Abx1/c3oi9ZaJvW22kAggq\ndzdA27UUYjWvx42w9menJwh/0jeQcTecIUd0d0rFcw/c1pvgMMl/Q73yzKgKYw==\n=zbHE\n-----END PGP PUBLIC KEY BLOCK-----\n-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBFsy23UBEACUKSphFEIEvNpy68VeW4Dt6qv+mU6am9a2AAl10JANLj1oqWX+\noYk3en1S6cVe2qehSL5DGVa3HMUZkP3dtbD4SgzXzxPodebPcr4+0QNWigkUisri\nXGL5SCEcOP30zDhZvg+4mpO2jMi7Kc1DLPzBBkgppcX91wa0L1pQzBcvYMPyV/Dh\nKbQHR75WdkP6OA2JXdfC94nxYq+2e0iPqC1hCP3Elh+YnSkOkrawDPmoB1g4+ft/\nxsiVGVy/W0ekXmgvYEHt6si6Y8NwXgnTMqxeSXQ9YUgVIbTpsxHQKGy76T5lMlWX\n4LCOmEVomBJg1SqF6yi9Vu8TeNThaDqT4/DddYInd0OO69s0kGIXalVgGYiW2HOD\nx2q5R1VGCoJxXomz+EbOXY+HpKPOHAjU0DB9MxbU3S248LQ69nIB5uxysy0PSco1\nsdZ8sxRNQ9Dw6on0Nowx5m6Thefzs5iK3dnPGBqHTT43DHbnWc2scjQFG+eZhe98\nEll/kb6vpBoY4bG9/wCG9qu7jj9Z+BceCNKeHllbezVLCU/Hswivr7h2dnaEFvPD\nO4GqiWiwOF06XaBMVgxA8p2HRw0KtXqOpZk+o+sUvdPjsBw42BB96A1yFX4jgFNA\nPyZYnEUdP6OOv9HSjnl7k/iEkvHq/jGYMMojixlvXpGXhnt5jNyc4GSUJQARAQAB\ntDNSZWQgSGF0LCBJbmMuIChhdXhpbGlhcnkga2V5KSA8c2VjdXJpdHlAcmVkaGF0\nLmNvbT6JAjkEEwECACMFAlsy23UCGwMHCwkIBwMCAQYVCAIJCgsEFgIDAQIeAQIX\ngAAKCRD3b2bD1AgnknqOD/9fB2ASuG2aJIiap4kK58R+RmOVM4qgclAnaG57+vjI\nnKvyfV3NH/keplGNRxwqHekfPCqvkpABwhdGEXIE8ILqnPewIMr6PZNZWNJynZ9i\neSMzVuCG7jDoGyQ5/6B0f6xeBtTeBDiRl7+Alehet1twuGL1BJUYG0QuLgcEzkaE\n/gkuumeVcazLzz7L12D22nMk66GxmgXfqS5zcbqOAuZwaA6VgSEgFdV2X2JU79zS\nBQJXv7NKc+nDXFG7M7EHjY3Rma3HXkDbkT8bzh9tJV7Z7TlpT829pStWQyoxKCVq\nsEX8WsSapTKA3P9YkYCwLShgZu4HKRFvHMaIasSIZWzLu+RZH/4yyHOhj0QB7XMY\neHQ6fGSbtJ+K6SrpHOOsKQNAJ0hVbSrnA1cr5+2SDfel1RfYt0W9FA6DoH/S5gAR\ndzT1u44QVwwp3U+eFpHphFy//uzxNMtCjjdkpzhYYhOCLNkDrlRPb+bcoL/6ePSr\n016PA7eEnuC305YU1Ml2WcCn7wQV8x90o33klJmEkWtXh3X39vYtI4nCPIvZn1eP\nVy+F+wWt4vN2b8oOdlzc2paOembbCo2B+Wapv5Y9peBvlbsDSgqtJABfK8KQq/jK\nYl3h5elIa1I3uNfczeHOnf1enLOUOlq630yeM/yHizz99G1g+z/guMh5+x/OHraW\niA==\n=+Gxh\n-----END PGP PUBLIC KEY BLOCK-----\n"
        }
EOF
    fi

    tee -a rhel-"${VERSION_ID%.*}".json << EOF
    ]
EOF

    # append comma for all arches except the last one
    if [ "$ARCH" != "x86_64" ]; then
        tee -a rhel-"${VERSION_ID%.*}".json << EOF
        ,
EOF
    fi
done

tee -a rhel-"${VERSION_ID%.*}".json << EOF
}
EOF

cp rhel-"${VERSION_ID%.*}".json rhel-"${VERSION_ID%.*}"-beta.json

# Create tests .repo file if REPO_URL is provided from ENV
# Otherwise osbuild-composer-tests.rpm will be downloaded from
# existing repositories
if [ -n "${REPO_URL+x}" ]; then
    JOB_NAME="${JOB_NAME:-${CI_JOB_ID}}"

    greenprint "📜 Amend dnf repository file"
    tee -a rhel"${VERSION_ID%.*}"internal.repo << EOF

[osbuild-composer-tests-multi-arch]
name=Tests ${JOB_NAME}
baseurl=${REPO_URL}
enabled=1
gpgcheck=0
# osbuild-composer repo priority is 5
priority=1
EOF

fi
