#!/usr/bin/env python3
"""Build a local-only, native Grafana dashboard. Never writes cluster resources."""
import copy
import json
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
DS = {"type": "prometheus", "uid": "prometheus"}
NODES = ["master", "m4neo", "worker1", "worker2"]
KSM = '(cluster_watch:ksm_healthy:bool == 1) and on() (time() - timestamp(cluster_watch:ksm_healthy:bool) < 120)'
MAPPINGS = [{"type": "value", "options": {
    "-1": {"text": "◇ 未取得", "color": "gray"},
    "0": {"text": "× 未Ready", "color": "red"},
    "1": {"text": "● 稼働確認", "color": "green"},
}}]


def fresh(selector):
    return f'(time() - min(timestamp({selector})) < 120)'


def guarded(expr, checks=(), ksm=True, fallback=-1):
    parts = [f'({expr})']
    if ksm:
        parts.append(f'({KSM})')
    parts.extend(f'({x})' for x in checks)
    return '(' + ' and on() '.join(parts) + f') or vector({fallback})'


def node_ready(node):
    s = f'kube_node_status_condition{{node="{node}",condition="Ready",status="true"}}'
    return guarded(f'max({s})', [f'count({s}) == 1', fresh(s)])


def pods_ready(regex, count):
    s = f'kube_pod_status_ready{{namespace="kube-system",condition="true",pod=~"{regex}"}}'
    return guarded(f'min({s})', [f'count({s}) == {count}', fresh(s)])


def deployment_ready(namespace, regex, count):
    labels = f'namespace="{namespace}",deployment=~"{regex}"'
    a = f'kube_deployment_status_replicas_available{{{labels}}}'
    d = f'kube_deployment_spec_replicas{{{labels}}}'
    return guarded(f'min({a} >= bool on(namespace,deployment) {d})',
                   [f'count({a}) == {count}', f'count({d}) == {count}', f'min({d}) > 0', fresh(a), fresh(d)])


def daemonset_ready(namespace, regex, count):
    labels = f'namespace="{namespace}",daemonset=~"{regex}"'
    a = f'kube_daemonset_status_number_ready{{{labels}}}'
    d = f'kube_daemonset_status_desired_number_scheduled{{{labels}}}'
    return guarded(f'min({a} >= bool on(namespace,daemonset) {d})',
                   [f'count({a}) == {count}', f'count({d}) == {count}', f'min({d}) > 0', fresh(a), fresh(d)])


def resource(node, memory=False):
    # Node-exporter series have instance; uname provides the stable nodename.
    info = f'node_uname_info{{nodename="{node}"}}'
    join = f'* on(instance) group_left(nodename) {info}'
    if memory:
        raw = f'100 * (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) {join}'
        checks = [f'min(node_memory_MemTotal_bytes {join}) > 0',
                  f'count(node_memory_MemAvailable_bytes {join}) == 1',
                  f'count(node_memory_MemTotal_bytes {join}) == 1']
        metrics = ['node_memory_MemAvailable_bytes', 'node_memory_MemTotal_bytes']
    else:
        raw = f'100 * (1 - avg by(instance)(rate(node_cpu_seconds_total{{mode="idle"}}[5m]))) {join}'
        capacity = f'kube_node_status_capacity{{node="{node}",resource="cpu",unit="core"}}'
        checks = [f'count({capacity}) == 1', fresh(capacity), f'max({capacity}) > 0',
                  f'count(node_cpu_seconds_total{{mode="idle"}} {join}) == scalar(max({capacity}))',
                  f'count(rate(node_cpu_seconds_total{{mode="idle"}}[5m]) {join}) == scalar(max({capacity}))']
        metrics = ['node_cpu_seconds_total{mode="idle"}']
    for metric in metrics:
        checks.append(f'(time() - min(timestamp({metric}) {join}) < 120)')
    checks += [f'count({info}) == 1', fresh(info),
               f'min(up{{job="node-exporter"}} {join}) == 1',
               f'count(up{{job="node-exporter"}} {join}) == 1',
               f'(time() - min(timestamp(up{{job="node-exporter"}}) {join}) < 120)']
    return guarded(f'avg({raw})', checks)


def target(expr, alias, ref, instant=True):
    return {"datasource": DS, "editorMode": "code", "expr": expr, "legendFormat": alias,
            "refId": ref, "range": not instant, "instant": instant, "format": "time_series"}


def panel(kind, title, x, y, w, h, targets=None):
    return {"type": kind, "title": title, "gridPos": {"x": x, "y": y, "w": w, "h": h},
            "datasource": DS, "targets": targets or [], "fieldConfig": {"defaults": {
                "color": {"mode": "thresholds"}, "noValue": "◇ 未取得",
                "thresholds": {"mode": "absolute", "steps": [{"color": "gray", "value": None},
                    {"color": "green", "value": 0}, {"color": "orange", "value": 75}, {"color": "red", "value": 90}]}},
                "overrides": []}, "options": {}}


def state_panel(title, expr, x, y, w=4, description=""):
    p = panel("stat", title, x, y, w, 4, [target(expr, title, "A")])
    p['description'] = description
    p['fieldConfig']['defaults'].update({"mappings": copy.deepcopy(MAPPINGS)})
    p['options'] = {"colorMode": "value", "graphMode": "none", "textMode": "value",
                    "justifyMode": "center", "reduceOptions": {"calcs": ["last"], "values": False},
                    "text": {"valueSize": 21}}
    return p
