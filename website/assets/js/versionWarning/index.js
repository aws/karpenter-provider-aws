export function versionWarning() {
  const viewingVersion = window.location.pathname.split('/')[1]
  if (viewingVersion) {
    const latestVersion = [
      ...document.querySelectorAll("#navbarVersionSelector .dropdown-item")
    ].map(el => el.dataset.docsVersion)[0]

    if (viewingVersion !== latestVersion) {
      const alertElement = document.createElement('div')
      alertElement.classList.add('alert', 'alert-warning')
      alertElement.innerHTML = `
        <h3>You are viewing Karpenter's <strong>${viewingVersion}</strong> documentation</h3>
        <p>
          Karpenter <strong>${viewingVersion}</strong> is not the latest stable release. 
          For up-to-date documentation, see the <a href="/${latestVersion}">latest version</a>.
        </p>
      `
      document.querySelector('.td-content').prepend(alertElement)
    }
  }
}
