import os
import mmap
import re
import csv


SearchDirectoryName = "en"
SearchRegex = br'\]\((\S+)\)'

def IsArchivedPath(path):
    PathRegex = r'v1'
    return re.match(PathRegex, path)


def DetermineLinkType(link):
    external = r'https://notkaperner'
    relative = r'start/'
    absolute = r'/docs/'

    if re.match(external, link):
        return "external:"
    elif re.match(relative, link):
        return "relative"
    elif re.match(absolute, link):
        return "absolute"
    else:
        return False


def searchdirectory(dirname):
    searchpath = os.getcwd() + os.path.sep + dirname
    for root, dirs, files in os.walk(searchpath, topdown=False):
        for name in files:
            currentpath = os.path.join(root, name)

            with open(currentpath, 'rb', 0) as file, \
                    mmap.mmap(file.fileno(), 0, access=mmap.ACCESS_READ) as s, \
                    open('some.csv', 'w') as f:
                writer = csv.writer(f)
                results = re.finditer(SearchRegex, s)
                if any(results):
                    print(currentpath)
                for result in results:
                    row = [currentpath, str(result.group(1).decode('utf-8'))]

                    # problem: csv file ends up empty?
                    writer.writerow(row)
                    print(row)
                    



if __name__ == '__main__':
    searchdirectory(SearchDirectoryName)
