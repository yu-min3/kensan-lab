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

