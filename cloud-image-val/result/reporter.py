import os


class Reporter:
    def __init__(self, junit_report_path):
        self.report_path = junit_report_path

    def generate_html_report(self, destination_path):
        os.system(f'junit2html {self.report_path} --report-matrix {destination_path}')
        print(f'HTML report generated: {destination_path}')
