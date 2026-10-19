#!/usr/bin/env python3
"""Provision a private, restartable GitHub download egress without restarting apps.

Secrets are generated in memory and sent over SSH stdin, never printed. Each
endpoint owns one dedicated systemd unit and config; existing Xray stays intact.
"""
import argparse
import hashlib
import json
import secrets
import shlex
import subprocess
import uuid

CONFIG = '/etc/sub2api-update-egress'
BINARY = '/usr/local/libexec/sub2api-update-egress/xray'
UNIT = 'sub2api-update-egress.service'
DOMAINS = ['domain:github.com', 'domain:githubusercontent.com']


def remote(host, command, data=None):
    result = subprocess.run(['ssh', '-o', 'BatchMode=yes', host, command],
                            input=data, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                            timeout=180)
    if result.returncode:
        raise RuntimeError('SSH operation failed; inspect the remote unit locally')
    return result.stdout


def write(host, path, data):
    remote(host, 'umask 077; cat > ' + shlex.quote(path), data)


def unit(server, source_ip, port):
    firewall = ''
    if server:
        rule = (f'INPUT -p tcp --dport {port} ! -s {source_ip} '
                '-m comment --comment sub2api-update-egress -j DROP')
        firewall = (f'ExecStartPre=+/bin/sh -c "iptables -C {rule} 2>/dev/null || iptables -I {rule}"\n'
                    f'ExecStopPost=+/bin/sh -c "iptables -D {rule} 2>/dev/null || true"\n')
    return (f'[Unit]\nDescription=Sub2API private GitHub download egress\n'
            'Wants=network-online.target\nAfter=network-online.target docker.service\n'
            '[Service]\nType=simple\nUser=nobody\nGroup=nogroup\n'
            + firewall + f'ExecStartPre={BINARY} run -test -config {CONFIG}/config.json\n'
            f'ExecStart={BINARY} run -config {CONFIG}/config.json\n'
            'Restart=always\nRestartSec=5\nNoNewPrivileges=true\nPrivateTmp=true\n'
            'ProtectSystem=strict\nProtectHome=true\nMemoryMax=128M\n'
            'RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX\n'
            '[Install]\nWantedBy=multi-user.target\n')


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ['server-ssh', 'client-ssh', 'server-ip', 'source-ip',
                 'server-name', 'client-listen', 'binary-sha256']:
        p.add_argument('--' + name, required=True)
    p.add_argument('--port', type=int, default=18444)
    p.add_argument('--proxy-port', type=int, default=18083)
    a = p.parse_args()
    import ipaddress
    for value in [a.server_ip, a.source_ip, a.client_listen]:
        ipaddress.ip_address(value)
    for host in [a.server_ssh, a.client_ssh]:
        remote(host, f'test ! -e {CONFIG} && test ! -e /etc/systemd/system/{UNIT}')
    binary = remote(a.server_ssh, 'cat /usr/local/bin/xray')
    if hashlib.sha256(binary).hexdigest() != a.binary_sha256:
        raise RuntimeError('Trusted binary checksum mismatch')
    keys = remote(a.server_ssh, '/usr/local/bin/xray x25519').decode().splitlines()
    keys = {k.strip().lower(): v.strip() for k, v in
            (line.split(':', 1) for line in keys if ':' in line)}
    private = next(v for k, v in keys.items() if k.startswith('private'))
    public = next(v for k, v in keys.items() if 'password' in k or k.startswith('public'))
    identity, short = str(uuid.uuid4()), secrets.token_hex(8)
    account = {'user': 'sub2api-update', 'pass': secrets.token_urlsafe(32)}
    log = {'access': 'none', 'error': 'none', 'loglevel': 'none'}
    server = {'log': log, 'inbounds': [{
        'listen': '0.0.0.0', 'port': a.port, 'protocol': 'vless',
        'settings': {'clients': [{'id': identity, 'email': 'sub2api-update',
                                  'flow': 'xtls-rprx-vision'}], 'decryption': 'none'},
        'streamSettings': {'network': 'tcp', 'security': 'reality', 'realitySettings': {
            'dest': '127.0.0.1:443', 'serverNames': [a.server_name],
            'privateKey': private, 'shortIds': [short]}}}],
        'outbounds': [{'tag': 'deny', 'protocol': 'blackhole'},
                      {'tag': 'github', 'protocol': 'freedom'}],
        'routing': {'rules': [{'type': 'field', 'domain': DOMAINS,
                              'port': '443', 'outboundTag': 'github'}]}}
    client = {'log': log, 'inbounds': [
        {'listen': address, 'port': a.proxy_port, 'protocol': 'http',
         'settings': {'accounts': [account]}}
        for address in dict.fromkeys(['127.0.0.1', a.client_listen])],
        'outbounds': [{'tag': 'github', 'protocol': 'vless', 'settings': {'vnext': [{
            'address': a.server_ip, 'port': a.port, 'users': [{
                'id': identity, 'encryption': 'none', 'flow': 'xtls-rprx-vision'}]}]},
            'streamSettings': {'network': 'tcp', 'security': 'reality', 'realitySettings': {
                'fingerprint': 'chrome', 'serverName': a.server_name,
                'password': public, 'shortId': short}}}]}
    # Prepare both endpoints completely before starting either one.
    for host, config, is_server in [(a.server_ssh, server, True), (a.client_ssh, client, False)]:
        remote(host, f'install -d -m 750 -g nogroup {CONFIG}; install -d -m 755 {BINARY.rsplit("/", 1)[0]}')
        write(host, BINARY, binary)
        write(host, CONFIG + '/config.json', json.dumps(config).encode())
        remote(host, f'chmod 755 {BINARY}; chown root:nogroup {CONFIG}/config.json; chmod 640 {CONFIG}/config.json; {BINARY} run -test -config {CONFIG}/config.json')
        write(host, '/etc/systemd/system/' + UNIT, unit(is_server, a.source_ip, a.port).encode())
        remote(host, f'chmod 644 /etc/systemd/system/{UNIT}; systemd-analyze verify /etc/systemd/system/{UNIT}')
    proxy = f'http://{account["user"]}:{account["pass"]}@{a.client_listen}:{a.proxy_port}'
    write(a.client_ssh, CONFIG + '/updater.env', f'UPDATE_PROXY_URL={proxy}\n'.encode())
    for host in [a.server_ssh, a.client_ssh]:
        remote(host, f'systemctl daemon-reload; systemctl enable --now {UNIT}; systemctl is-active --quiet {UNIT}')
    print('Private egress installed and enabled. Application configuration has not been changed.')


if __name__ == '__main__':
    main()
