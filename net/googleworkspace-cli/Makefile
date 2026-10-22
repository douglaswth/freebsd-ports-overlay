PORTNAME=	googleworkspace-cli
DISTVERSIONPREFIX=	v
DISTVERSION=	0.22.5
CATEGORIES=	net

MAINTAINER=	douglas@douglasthrift.net
COMMENT=	Command-line tool for Google Workspace APIs
WWW=		https://github.com/googleworkspace/cli

LICENSE=	APACHE20
LICENSE_FILE=	${WRKSRC}/LICENSE

USES=		cargo
USE_GITHUB=	yes
GH_ACCOUNT=	googleworkspace
GH_PROJECT=	cli

CARGO_INSTALL_PATH=	crates/google-workspace-cli

PLIST_FILES=	bin/gws

.include <bsd.port.mk>
