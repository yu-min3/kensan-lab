"""Live application state for the production entry; sample dashboards stay separate."""
from common import DS

# Matches the PodMonitor. Keep the application project separate from platform apps.
INFO = 'argocd_app_info{job="argocd-application-controller",namespace="argocd",project="app-project"}'
UP = 'up{job="argocd-application-controller",namespace="argocd"}'
ARGO = 'https://argocd.platform.yu-min3.com/applications'
FRESH = f'((min({UP}) == 1) and (time() - min(timestamp({UP})) < 120))'
VALID = f'(({INFO} == 1) and (time() - timestamp({INFO}) < 120)) and on() {FRESH}'
OBSERVED = f'max by(name,sync_status,health_status) (timestamp({INFO}) and ({VALID})) * 1000'
# Keep metadata and timestamp from the same sample. Never retain old Healthy labels
# after scrape failure, even while Prometheus still has the old sample in lookback.
LIVE = f'label_replace(({OBSERVED}), "application_url", "{ARGO}/argocd/$1", "name", "(.+)")'
MISSING = 'vector(-1) unless on() (' + OBSERVED + ')'
for label, value in [('name', 'アプリ情報を取得できません'), ('sync_status', '未取得'),
                     ('health_status', '未取得'), ('application_url', ARGO)]:
    MISSING = f'label_replace(({MISSING}), "{label}", "{value}", "", "")'
EXPR = f'label_replace(({LIVE} or {MISSING}), "route_status", "未計測", "", "")'


def override(name, **props):
    return {'matcher': {'id': 'byName', 'options': name},
            'properties': [{'id': k, 'value': v} for k, v in props.items()]}


def mappings(states):
    return [{'type': 'value', 'options': {
        value: {'text': text, 'color': color} for value, text, color in states}}]


def application_panel():
    return {
        'id': 8, 'type': 'table', 'title': 'アプリの状態',
        'description': 'Argo CDのapp-projectを実測。SyncはGitとの一致、Healthはリソースの健全性です。'
                       '利用者のHTTP経路は未計測です。収集失敗・120秒以上古い値・情報なしは未取得として扱います。'
                       'アプリ名からArgo CDの実リソースへ進めます。',
        'gridPos': {'x': 0, 'y': 17, 'w': 24, 'h': 7}, 'datasource': DS,
        'targets': [{'refId': 'A', 'datasource': DS, 'expr': EXPR, 'format': 'table',
                     'instant': True, 'range': False, 'editorMode': 'code'}],
        'links': [{'title': 'Argo CDの全Application', 'url': ARGO, 'targetBlank': True}],
        'transformations': [{'id': 'organize', 'options': {
            'excludeByName': {'Time': True},
            'indexByName': {'name': 0, 'sync_status': 1, 'health_status': 2,
                            'route_status': 3, 'Value': 4, 'application_url': 5},
            'renameByName': {'name': 'アプリ', 'sync_status': 'Sync', 'health_status': 'Health',
                             'route_status': '経路', 'Value': '観測時刻'}}}],
        'fieldConfig': {'defaults': {'noValue': '未取得', 'color': {'mode': 'fixed', 'fixedColor': 'text'},
                                    'custom': {'align': 'left', 'cellOptions': {'type': 'auto'}}},
                        'overrides': [
            override('アプリ', **{'custom.width': 235, 'links': [{
                'title': 'Argo CDでリソースを確認', 'url': '${__data.fields["application_url"]}', 'targetBlank': True}]}),
            override('Sync', **{'custom.width': 115, 'custom.cellOptions': {'type': 'color-text'},
                               'mappings': mappings([('Synced', 'Synced', 'text'), ('OutOfSync', 'OutOfSync', 'orange'),
                                                     ('Unknown', 'Unknown', 'gray'), ('未取得', '◇ 未取得', 'gray')])}),
            override('Health', **{'custom.width': 125, 'custom.cellOptions': {'type': 'color-text'},
                                 'mappings': mappings([('Healthy', '● Healthy', 'green'), ('Degraded', '× Degraded', 'red'),
                                    ('Progressing', '◷ Progressing', 'blue'), ('Missing', '× Missing', 'red'),
                                    ('Suspended', 'Ⅱ Suspended', 'gray'), ('Unknown', '◇ Unknown', 'gray'),
                                    ('未取得', '◇ 未取得', 'gray')])}),
            override('経路', **{'custom.width': 100, 'custom.cellOptions': {'type': 'color-text'},
                               'mappings': mappings([('未計測', '— 未計測', 'gray')])}),
            override('観測時刻', **{'unit': 'time:YYYY-MM-DD HH:mm:ss', 'custom.width': 180,
                                   'mappings': [{'type': 'value', 'options': {'-1': {'text': '—', 'color': 'gray'}}}]}),
            override('application_url', **{'custom.hidden': True}),
        ]},
        'options': {'showHeader': True, 'cellHeight': 'sm', 'sortBy': [{'displayName': 'アプリ', 'desc': False}],
                    'footer': {'show': False}},
    }
