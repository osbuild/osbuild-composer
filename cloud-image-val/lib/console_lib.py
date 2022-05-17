def color_print(text):
    print('\033[95m' + text + '\033[0m')


def print_divider(text):
    divider_length = 75
    free_spaces = int((divider_length / 2) - (len(text) / 2))

    color_print('-' * divider_length)
    color_print(' ' * free_spaces + text)
    color_print('-' * divider_length)
