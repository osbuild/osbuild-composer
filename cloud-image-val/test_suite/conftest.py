import json
import pytest

from py.xml import html
from pytest_html import extras


@pytest.fixture(autouse=True)
def instance_data(host):
    return __get_instance_data_from_json(key_to_find='public_dns',
                                         value_to_find=host.backend.hostname)


def __get_instance_data_from_json(key_to_find, value_to_find):
    # TODO: Pass this hardcoded path to a config file and read from there.
    with open('/tmp/instances.json', 'r') as f:
        instances_json_data = json.load(f)
    for instance in instances_json_data.values():
        if key_to_find in instance.keys() and instance[key_to_find] == value_to_find:
            return instance


@pytest.fixture(autouse=True)
def html_report_links(extra, instance_data):
    extra.append(extras.json(instance_data))


def pytest_configure(config):
    config._metadata['GitHub'] = 'https://github.com/osbuild/cloud-image-val'


def pytest_html_report_title(report):
    report.title = 'Cloud Image Validation Report'


def pytest_html_results_table_header(cells):
    cells.insert(2, html.th('Image Reference', class_='sortable'))
    cells.insert(3, html.th('Description'))
    cells.insert(4, html.th('Error Message'))


def pytest_html_results_table_row(report, cells):
    cells.insert(2, html.td(getattr(report, 'image_reference', '')))
    cells.insert(3, html.td(getattr(report, 'description', ''),
                            style='white-space:pre-line; word-wrap:break-word'))
    cells.insert(4, html.td(getattr(report, 'error_message', ''),
                            style='white-space:pre-line; word-wrap:break-word'))


@pytest.hookimpl(hookwrapper=True)
def pytest_runtest_makereport(item, call):
    outcome = yield
    report = outcome.get_result()

    # Set the test cases "Duration" format
    setattr(report, 'duration_formatter', '%S.%f sec')

    # Fill "Image Reference" column
    instance = item.funcargs['instance_data']
    if instance:
        image_ref = instance['ami'] if 'ami' in instance else instance['image']
        report.image_reference = str(image_ref)

    # Fill "Description" column
    description_text = __truncate_text(str(item.function.__doc__), 200)
    report.description = description_text

    # Fill "Error Message" column
    setattr(item, 'rep_' + report.when, report)
    report.error_message = str(call.excinfo.value) if call.excinfo else ''
    if report.when == 'teardown':
        message = item.rep_call.error_message.split(' assert ')[0]
        message = __truncate_text(message, 120)

        report.error_message = message


def __truncate_text(text, max_chars):
    if len(text) > max_chars:
        text = f'{text[:max_chars]} [...]'
    return text
