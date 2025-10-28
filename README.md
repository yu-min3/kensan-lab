# cluster settings

## masterノードで実行
### 1. kubeadm init
```
sudo kubeadm init \
  --apiserver-advertise-address=<MasterノードのIPアドレス> \
  --control-plane-endpoint=<MasterノードのIPアドレスまたはホスト名> \
  --pod-network-cidr=<cidr範囲> \ # 10.244.0.0/16などがよく使われる範囲。
  --ignore-preflight-errors=Mem # メモリが少ない環境でのバグ回避
```

### 2. kubeconfigの設定
出力分に従ってkubeconfigを設定。
```
# 1. .kube ディレクトリを作成 (既に存在する場合はスキップ)
mkdir -p $HOME/.kube

# 2. クラスター管理者設定ファイル（admin.conf）をホームディレクトリにコピー
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config

# 3. コピーしたファイルの所有者を現在のユーザーに変更
sudo chown $(id -u):$(id -g) $HOME/.kube/config
```

開発マシンに設定したい場合はscpコマンドで移送
```
scp <Masterノードのユーザー名>@<masterノード>:/etc/kubernetes/admin.conf ~/Downloads/kubeconfig-master.yaml
```

### 3. workernodeを酸化させる
master nodeで以下コマンドを実行
```
sudo kubeadm token create --print-join-command
```

出力されたコマンドを参加させたいworker nodeで実行

# boostrap
## cilium
### 1. Helmリポジトリを追加
```
helm repo add cilium https://helm.cilium.io/
helm repo update
```

### 2. YAMLマニフェストを生成し、platform-configリポジトリに保存
### オプション: kubeProxyReplacement=true (kube-proxyの置き換え。e-BPFによる高速ネットワーキングのため採用)
### k8s.cluster.cidr=10.244.0.0/16 (Podネットワーク範囲の指定)
```
helm template cilium cilium/cilium \
  --namespace kube-system \
  --create-namespace \
  --set kubeProxyReplacement=true \
  --set k8s.cluster.cidr=<cidr範囲> \
  --set ipam.mode=kubernetes \
  > base-infra/cilium/cilium.yaml
  ```

### 3.　接続テスト
```
cilium connectivity test
```

## metallb
### 1. 必要なyamlファイルを揃える
```
curl -L https://raw.githubusercontent.com/metallb/metallb/<metallbのバージョン>>/config/manifests/metallb-native.yaml  
 > base-infra/metallb/metallb-controller.yaml
kubectl create namespace metallb-system --dry-run=client -o yaml > base-infra/metallb/namespace.yaml
```
ipaddresspoolはリポジトリを参考にipを選択

### 2. 適用
```
k apply -f base-infra/metallb/namespace.yaml
k apply -f base-infra/metallb/metallb-controller.yaml 
# ipaddresspoolはcontrollerの作成が完了してから
k apply -f base-infra/metallb/ipaddresspool.yaml
```

# secret管理
```
# GHCRからのimagepull用secretのyamlファイルを作成
kubectl create secret docker-registry ghcr-pull-secret \
  --docker-server=ghcr.io \
  --docker-username=<GitHubユーザー名> \
  --docker-password=<PAT> \
  --docker-email=<githubメールアドレス> \
  --namespace=<利用するnamespace> \
  --dry-run=client \
  -o yaml > ghcr-secret-raw.yaml

# sealed secretに変換
kubeseal --format=yaml < ghcr-secret-raw.yaml > platform-config/base-infra/secrets/ghcr-pull-secret.yaml
```