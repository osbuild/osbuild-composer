import pytest

from result.reporter import Reporter


class TestReporter:
    test_junit_report_path = '/test/path/to/junit/report.xml'

    @pytest.fixture
    def reporter(self):
        return Reporter(self.test_junit_report_path)

    def test_generate_html_report(self, mocker, reporter):
        test_destination_path = 'test/path/to/report.html'
        mock_os_system = mocker.patch('os.system')
        mock_print = mocker.patch('builtins.print')

        reporter.generate_html_report(test_destination_path)

        mock_os_system.assert_called_once_with(f'junit2html {self.test_junit_report_path} '
                                               f'--report-matrix {test_destination_path}')
        mock_print.assert_called_once_with(f'HTML report generated: {test_destination_path}')
