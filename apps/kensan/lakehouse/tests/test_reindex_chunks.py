"""
reindex_pending_chunks のユニットテスト
kensan-ai HTTP 呼び出しを mock してテスト
"""

from unittest.mock import patch, MagicMock

from pipelines.maintenance.reindex_chunks import reindex_pending_chunks


def _mock_response(json_data: dict, status_code: int = 200) -> MagicMock:
    resp = MagicMock()
    resp.status_code = status_code
    resp.json.return_value = json_data
    resp.raise_for_status.return_value = None
    return resp


@patch("pipelines.maintenance.common.requests.post")
def test_reindex_pending_chunks_success(mock_post):
    """正常レスポンスが返る"""
    mock_post.return_value = _mock_response({
        "users_processed": 2,
        "total_processed": 10,
        "total_chunks": 45,
    })

    result = reindex_pending_chunks("http://kensan-ai:8089", batch_size=50)

    assert result["users_processed"] == 2
    assert result["total_processed"] == 10
    assert result["total_chunks"] == 45

    mock_post.assert_called_once_with(
        "http://kensan-ai:8089/api/v1/admin/reindex-pending",
        params={"batch_size": 50},
        timeout=300,
    )


@patch("pipelines.maintenance.common.requests.post")
def test_reindex_pending_chunks_no_pending(mock_post):
    """pending ノートがない場合"""
    mock_post.return_value = _mock_response({
        "users_processed": 0,
        "total_processed": 0,
        "total_chunks": 0,
    })

    result = reindex_pending_chunks("http://kensan-ai:8089")

    assert result["users_processed"] == 0
    assert result["total_processed"] == 0
    assert result["total_chunks"] == 0


@patch("pipelines.maintenance.common.requests.post")
def test_reindex_pending_chunks_custom_batch_size(mock_post):
    """batch_size パラメータが渡される"""
    mock_post.return_value = _mock_response({
        "users_processed": 1,
        "total_processed": 5,
        "total_chunks": 20,
    })

    reindex_pending_chunks("http://localhost:8089", batch_size=100)

    mock_post.assert_called_once_with(
        "http://localhost:8089/api/v1/admin/reindex-pending",
        params={"batch_size": 100},
        timeout=300,
    )


@patch("pipelines.maintenance.common.requests.post")
def test_reindex_pending_chunks_http_error(mock_post):
    """HTTP エラーが raise される"""
    import requests

    mock_post.return_value = MagicMock()
    mock_post.return_value.raise_for_status.side_effect = requests.HTTPError("503")

    try:
        reindex_pending_chunks("http://kensan-ai:8089")
        assert False, "Should have raised"
    except requests.HTTPError:
        pass
