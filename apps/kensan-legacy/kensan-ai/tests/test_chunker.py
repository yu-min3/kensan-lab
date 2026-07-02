"""Tests for kensan_ai.indexing.chunker — チャンク分割のユニットテスト。"""

import pytest

from kensan_ai.indexing.chunker import (
    Chunk,
    chunk_code,
    chunk_content,
    chunk_drawio,
    chunk_markdown,
    estimate_tokens,
)


class TestEstimateTokens:
    """トークン数推定のテスト。"""

    def test_empty_string(self):
        """空文字列 → 0"""
        assert estimate_tokens("") == 0

    def test_none_like_empty(self):
        """Noneではなく空文字列のケース"""
        assert estimate_tokens("") == 0

    def test_english_text(self):
        """英語テキスト → 文字数 / 2.5 の推定値"""
        text = "Hello world, this is a test."  # 27 chars → 27/2.5 = 10.8 → 10
        result = estimate_tokens(text)
        assert result == int(len(text) / 2.5)

    def test_japanese_text(self):
        """日本語テキスト → 推定値検証"""
        text = "これはテストです。日本語のトークン推定。"  # 18 chars
        result = estimate_tokens(text)
        assert result == int(len(text) / 2.5)

    def test_minimum_is_1(self):
        """短いテキストでも最小値は1"""
        assert estimate_tokens("a") == 1
        assert estimate_tokens("ab") == 1


class TestChunkContent:
    """chunk_content ルーティングのテスト。"""

    def test_markdown_routes_to_chunk_markdown(self):
        """markdown → chunk_markdownに委譲"""
        result = chunk_content("# Hello\nWorld", "markdown")
        assert len(result) >= 1
        assert result[0].text == "# Hello\nWorld"

    def test_code_routes_to_chunk_code(self):
        """code → chunk_codeに委譲"""
        result = chunk_content("def foo(): pass", "code")
        assert len(result) >= 1
        assert "def foo" in result[0].text

    def test_drawio_routes_to_chunk_drawio(self):
        """drawio → chunk_drawioに委譲"""
        xml = '<mxGraphModel><root><mxCell value="Test Label"/></root></mxGraphModel>'
        result = chunk_content(xml, "drawio")
        assert len(result) >= 1
        assert "Test Label" in result[0].text

    def test_image_returns_empty(self):
        """image → 空リスト"""
        result = chunk_content("binary data", "image")
        assert result == []

    def test_pdf_returns_empty(self):
        """pdf → 空リスト"""
        result = chunk_content("pdf content", "pdf")
        assert result == []

    def test_empty_string_returns_empty(self):
        """空文字列 → 空リスト"""
        assert chunk_content("", "markdown") == []

    def test_none_text_returns_empty(self):
        """Noneテキスト → 空リスト"""
        assert chunk_content(None, "markdown") == []

    def test_whitespace_only_returns_empty(self):
        """空白のみ → 空リスト"""
        assert chunk_content("   \n  ", "markdown") == []

    def test_unknown_content_type_returns_empty(self):
        """未知のcontent_type → 空リスト"""
        assert chunk_content("some text", "unknown") == []


class TestChunkMarkdown:
    """Markdown チャンク分割のテスト。"""

    def test_short_text_no_headings(self):
        """見出しなし短文 → 1チャンク"""
        result = chunk_markdown("Just a short paragraph.")
        assert len(result) == 1
        assert result[0].index == 0
        assert result[0].text == "Just a short paragraph."

    def test_split_by_headings(self):
        """h1/h2/h3で分割 → 見出し数に応じたチャンク"""
        text = "# Section 1\nContent 1\n\n## Section 2\nContent 2\n\n### Section 3\nContent 3"
        result = chunk_markdown(text)
        assert len(result) == 3
        assert result[0].text.startswith("# Section 1")
        assert result[1].text.startswith("## Section 2")
        assert result[2].text.startswith("### Section 3")

    def test_long_section_split_by_paragraph(self):
        """500トークン超セクション → 段落で追加分割"""
        # 500 tokens ≈ 1250 chars at 2.5 chars/token
        long_para1 = "A" * 700
        long_para2 = "B" * 700
        text = f"# Long Section\n{long_para1}\n\n{long_para2}"
        result = chunk_markdown(text, max_tokens=500)
        assert len(result) >= 2

    def test_text_before_first_heading(self):
        """見出し前のテキスト → 最初のチャンクに含まれる"""
        text = "Preamble text\n\n# First Heading\nContent"
        result = chunk_markdown(text)
        assert len(result) == 2
        assert result[0].text == "Preamble text"
        assert result[1].text.startswith("# First Heading")

    def test_blank_lines_only(self):
        """空行のみ → 空リスト"""
        result = chunk_markdown("\n\n\n")
        assert result == []

    def test_chunk_indices_sequential(self):
        """チャンクのindexが0から連番"""
        text = "# A\nContent\n\n## B\nContent\n\n## C\nContent"
        result = chunk_markdown(text)
        for i, chunk in enumerate(result):
            assert chunk.index == i

    def test_chunk_has_token_count(self):
        """各チャンクにtoken_countが設定される"""
        result = chunk_markdown("# Test\nSome content here")
        assert len(result) == 1
        assert result[0].token_count > 0


class TestChunkCode:
    """コードチャンク分割のテスト。"""

    def test_short_code_single_chunk(self):
        """短いコード → 1チャンク"""
        code = "def hello():\n    print('hello')"
        result = chunk_code(code)
        assert len(result) == 1
        assert result[0].text == code

    def test_long_code_multiple_chunks(self):
        """長いコード → 複数チャンクに分割"""
        # 500 tokens ≈ 1250 chars
        code = "x = 1\n" * 300  # ~1800 chars
        result = chunk_code(code, max_tokens=500)
        assert len(result) >= 2

    def test_overlap_between_chunks(self):
        """overlap確認 → 連続チャンクの末尾/先頭が重複"""
        # Make code long enough for multiple chunks
        code = "".join(f"line_{i} = {i}\n" for i in range(200))
        result = chunk_code(code, max_tokens=100, overlap_tokens=20)
        assert len(result) >= 2
        # Check that the end of chunk 0 overlaps with start of chunk 1
        tail_of_first = result[0].text[-50:]
        head_of_second = result[1].text[:50]
        # There should be some common substring
        assert any(
            tail_of_first[i:i+10] in head_of_second
            for i in range(len(tail_of_first) - 10)
        ), "Expected overlap between consecutive chunks"

    def test_chunk_indices_sequential(self):
        """チャンクのindexが連番"""
        code = "x = 1\n" * 300
        result = chunk_code(code, max_tokens=100)
        for i, chunk in enumerate(result):
            assert chunk.index == i


class TestChunkDrawio:
    """Drawio XML チャンク分割のテスト。"""

    def test_valid_drawio_extracts_labels(self):
        """有効なdrawio XML → ラベル抽出して1チャンク"""
        xml = """<mxGraphModel>
          <root>
            <mxCell value="Start"/>
            <mxCell value="Process"/>
            <mxCell value="End"/>
          </root>
        </mxGraphModel>"""
        result = chunk_drawio(xml)
        assert len(result) == 1
        assert "Start" in result[0].text
        assert "Process" in result[0].text
        assert "End" in result[0].text

    def test_html_tags_removed_from_labels(self):
        """ラベルにHTMLタグ → タグ除去"""
        xml = '<mxGraphModel><root><mxCell value="&lt;b&gt;Bold&lt;/b&gt; text"/></root></mxGraphModel>'
        result = chunk_drawio(xml)
        assert len(result) == 1
        assert "<b>" not in result[0].text
        assert "Bold" in result[0].text

    def test_elements_without_value_skipped(self):
        """value属性なしの要素 → スキップ"""
        xml = """<mxGraphModel>
          <root>
            <mxCell id="0"/>
            <mxCell value="Only Label"/>
            <mxCell style="rounded"/>
          </root>
        </mxGraphModel>"""
        result = chunk_drawio(xml)
        assert len(result) == 1
        assert result[0].text == "Only Label"

    def test_invalid_xml_fallback(self):
        """不正XML → フォールバック（プレーンテキストとして1チャンク）"""
        bad_xml = "this is not xml <unclosed"
        result = chunk_drawio(bad_xml)
        assert len(result) == 1
        assert result[0].text == bad_xml

    def test_empty_labels_only(self):
        """空ラベルのみ → 空リスト"""
        xml = '<mxGraphModel><root><mxCell value=""/><mxCell value="   "/></root></mxGraphModel>'
        result = chunk_drawio(xml)
        assert result == []

    def test_single_chunk_returned(self):
        """複数ラベルでも常に1チャンク"""
        xml = """<mxGraphModel><root>
            <mxCell value="A"/>
            <mxCell value="B"/>
            <mxCell value="C"/>
        </root></mxGraphModel>"""
        result = chunk_drawio(xml)
        assert len(result) == 1
        assert result[0].index == 0
