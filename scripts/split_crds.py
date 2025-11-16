#!/usr/bin/env python3
import sys
import re

def split_yaml_file(input_file, crds_file, resources_file):
    """Split YAML file into CRDs and other resources"""
    
    with open(input_file, 'r', encoding='utf-8') as f:
        content = f.read()
    
    # Split by document separator
    documents = re.split(r'^---\s*$', content, flags=re.MULTILINE)
    
    crds = []
    resources = []
    
    for doc in documents:
        doc = doc.strip()
        if not doc:
            continue
            
        # Check if this document is a CRD
        if re.search(r'^kind:\s*CustomResourceDefinition\s*$', doc, flags=re.MULTILINE):
            crds.append(doc)
        else:
            resources.append(doc)
    
    # Write CRDs
    with open(crds_file, 'w', encoding='utf-8') as f:
        f.write('---\n')
        f.write('\n---\n'.join(crds))
        if crds:
            f.write('\n')
    
    # Write other resources
    with open(resources_file, 'w', encoding='utf-8') as f:
        f.write('---\n')
        f.write('\n---\n'.join(resources))
        if resources:
            f.write('\n')
    
    return len(crds), len(resources)

if __name__ == '__main__':
    if len(sys.argv) != 4:
        print(f"Usage: {sys.argv[0]} <input_file> <crds_file> <resources_file>")
        sys.exit(1)
    
    input_file = sys.argv[1]
    crds_file = sys.argv[2]
    resources_file = sys.argv[3]
    
    crd_count, resource_count = split_yaml_file(input_file, crds_file, resources_file)
    print(f"Split complete:")
    print(f"  CRDs: {crd_count} resources → {crds_file}")
    print(f"  Other resources: {resource_count} → {resources_file}")

